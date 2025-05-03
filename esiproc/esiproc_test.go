package esiproc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/nussjustin/esi"
	"github.com/nussjustin/esi/esiproc"
)

var (
	errInvalid       = errors.New("invalid input")
	errInterpolation = errors.New("interpolation failed")
)

type testEnv struct{}

func (t testEnv) Eval(_ context.Context, expr string) (any, error) {
	switch expr {
	case "false":
		return false, nil
	case "null":
		return nil, nil
	case "true":
		return true, nil
	default:
		return nil, errInvalid
	}
}

func (t testEnv) Interpolate(_ context.Context, s string) (string, error) {
	if strings.Contains(s, "$(ERROR)") {
		return "", errInterpolation
	}
	s = strings.ReplaceAll(s, "$(VAR1)", "var 1")
	s = strings.ReplaceAll(s, "$(VAR2)", "var 2")
	return s, nil
}

func parseESI(input string) ([]esi.Node, error) {
	nodes := make([]esi.Node, 0, 32)

	for node, err := range esi.NewParser(strings.NewReader(input)).All {
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

func TestProcessor(t *testing.T) {
	testCases := []struct {
		Name       string
		Opts       []esiproc.ProcessorOpt
		Input      string
		InputNodes []esi.Node
		Expected   string
		Error      error
	}{
		{
			Name: "attempt outside try",
			InputNodes: []esi.Node{
				&esi.AttemptElement{
					Position: esi.Position{Start: 5, End: 18},
				},
			},
			Error: &esiproc.UnexpectedElementError{
				Element: &esi.AttemptElement{
					Position: esi.Position{Start: 5, End: 18},
				},
			},
		},
		{
			Name: "choose first when",
			Input: `
				<esi:choose>
					<esi:when test="true">one</esi:when>
					<esi:when test="false">two</esi:when>
					<esi:otherwise>otherwise</esi:otherwise>
				</esi:choose>
			`,
			Expected: "one",
		},
		{
			Name: "choose second when",
			Input: `
				<esi:choose>
					<esi:when test="false">one</esi:when>
					<esi:when test="true">two</esi:when>
					<esi:otherwise>otherwise</esi:otherwise>
				</esi:choose>
			`,
			Expected: "two",
		},
		{
			Name: "choose otherwise",
			Input: `
				<esi:choose>
					<esi:when test="false">one</esi:when>
					<esi:when test="false">two</esi:when>
					<esi:otherwise>otherwise</esi:otherwise>
				</esi:choose>
			`,
			Expected: "otherwise",
		},
		{
			Name: "choose no otherwise",
			Input: `
				<esi:choose>
					<esi:when test="false">one</esi:when>
					<esi:when test="false">two</esi:when>
				</esi:choose>
			`,
			Expected: "",
		},
		{
			Name: "choose without test func",
			Opts: []esiproc.ProcessorOpt{
				esiproc.WithEnv(nil),
			},
			Input: `
				<esi:choose>
					<esi:when test="true">one</esi:when>
					<esi:when test="true">two</esi:when>
				</esi:choose>
			`,
			Error: &esiproc.UnsupportedElementError{
				Element: &esi.WhenElement{Position: esi.Position{Start: 23, End: 59}},
			},
		},
		{
			Name:     "comment",
			Input:    `before <esi:comment text="some comment"/> after`,
			Expected: `before  after`,
		},
		{
			Name: "except outside try",
			InputNodes: []esi.Node{
				&esi.ExceptElement{
					Position: esi.Position{Start: 5, End: 18},
				},
			},
			Error: &esiproc.UnexpectedElementError{
				Element: &esi.ExceptElement{
					Position: esi.Position{Start: 5, End: 18},
				},
			},
		},
		{
			Name:     "include",
			Input:    `before <esi:include src="/test"/> after`,
			Expected: `before {"url":"/test"} after`,
		},
		{
			Name:     "include with alt",
			Input:    `before <esi:include src="/test" alt="/panic"/> after`,
			Expected: `before {"url":"/test"} after`,
		},
		{
			Name:     "include with meta",
			Input:    `before <esi:include src="/test" alt="/panic" attr1="value1" ns:attr2="value2"/> after`,
			Expected: `before {"meta":{"attr1":"value1","ns:attr2":"value2"},"url":"/test"} after`,
		},
		{
			Name:  "include error",
			Input: `before <esi:include src="/error"/> after`,
			Error: errInvalid,
		},
		{
			Name:     "include error with alt",
			Input:    `before <esi:include src="/error" alt="/alt"/> after`,
			Expected: `before {"url":"/alt"} after`,
		},
		{
			Name:  "include error with alt error",
			Input: `before <esi:include src="/error" alt="/error"/> after`,
			Error: errInvalid,
		},
		{
			Name:     "include error with alt error and onerror=continue",
			Input:    `before <esi:include src="/error" alt="/error" onerror="continue"/> after`,
			Expected: `before  after`,
		},
		{
			Name:     "include error with onerror=continue",
			Input:    `before <esi:include src="/error" onerror="continue"/> after`,
			Expected: `before  after`,
		},
		{
			Name: "include without include func",
			Opts: []esiproc.ProcessorOpt{
				esiproc.WithIncludeFunc(nil),
			},
			Input: `before <esi:include src="/included"/> after`,
			Error: &esiproc.UnsupportedElementError{
				Element: &esi.IncludeElement{Position: esi.Position{Start: 7, End: 37}},
			},
		},
		{
			Name:     "include with variable in src",
			Input:    `<esi:include src="/$(VAR1)"/>`,
			Expected: `{"url":"/var 1"}`,
		},
		{
			Name:     "include with variable in alt",
			Input:    `<esi:include src="/error" alt="/$(VAR2)"/>`,
			Expected: `{"url":"/var 2"}`,
		},
		{
			Name: "include with unsupported variable",
			Opts: []esiproc.ProcessorOpt{
				esiproc.WithEnv(nil),
			},
			Input:    `<esi:include src="/$(VAR1)"/>`,
			Expected: `{"url":"/$(VAR1)"}`,
		},
		{
			Name:  "include with failed interpolation",
			Input: `<esi:include src="/$(ERROR)"/>`,
			Error: errInterpolation,
		},
		{
			Name: "otherwise outside choose",
			InputNodes: []esi.Node{
				&esi.OtherwiseElement{
					Position: esi.Position{Start: 5, End: 18},
				},
			},
			Error: &esiproc.UnexpectedElementError{
				Element: &esi.OtherwiseElement{
					Position: esi.Position{Start: 5, End: 18},
				},
			},
		},
		{
			Name:     "remove",
			Input:    `before <esi:remove> inside </esi:remove> after`,
			Expected: `before  after`,
		},
		{
			Name: "try",
			Input: `
				<esi:try>
					<esi:attempt><esi:include src="/attempt"/></esi:attempt>
					<esi:except><esi:include src="/panic"/></esi:except>
				</esi:try>
			`,
			Expected: `{"url":"/attempt"}`,
		},
		{
			Name: "try with failed include",
			Input: `
				<esi:try>
					<esi:attempt><esi:include src="/error"/></esi:attempt>
					<esi:except><esi:include src="/except"/></esi:except>
				</esi:try>
			`,
			Expected: `{"url":"/except"}`,
		},
		{
			Name: "try with failed include with alt",
			Input: `
				<esi:try>
					<esi:attempt><esi:include src="/error" alt="/alt"/></esi:attempt>
					<esi:except><esi:include src="/panic"/></esi:except>
				</esi:try>
			`,
			Expected: `{"url":"/alt"}`,
		},
		{
			Name: "try with failed include with onerror=continue",
			Input: `
				<esi:try>
					<esi:attempt><esi:include src="/error" onerror="continue"/></esi:attempt>
					<esi:except><esi:include src="/except"/></esi:except>
				</esi:try>
			`,
			Expected: ``,
		},
		{
			Name: "vars",
			Input: `
				<esi:vars>
					hello world
				</esi:vars>
			`,
			Error: &esiproc.UnsupportedElementError{
				Element: &esi.VarsElement{Position: esi.Position{Start: 5, End: 48}},
			},
		},
		{
			Name: "when outside choose",
			InputNodes: []esi.Node{
				&esi.WhenElement{
					Position: esi.Position{Start: 5, End: 18},
				},
			},
			Error: &esiproc.UnexpectedElementError{
				Element: &esi.WhenElement{
					Position: esi.Position{Start: 5, End: 18},
				},
			},
		},

		{
			Name: "complex",
			Input: `
				<esi:choose>
					<esi:when test="false">never</esi:when>
					<esi:when test="true">
						before comment
						<esi:comment text="some comment"/>
						after comment
						<p>some html</p>
						<esi:try>
							<esi:attempt>
								attempt start
								<esi:include src="/error"/>
								attempt end
							</esi:attempt>
							<esi:except>
								except start
								<esi:include src="/error" alt="hello world"/>
								except end
							</esi:except>
						</esi:try>
						before remove
						<esi:remove>
							<esi:include src="/panic"/>
						</esi:remove>
						after remove
					</esi:when>
				</esi:choose>
			`,
			Expected: `before comment
						
						after comment
						<p>some html</p>
						
								except start
								{"url":"hello world"}
								except end
							
						before remove
						
						after remove`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			nodes := testCase.InputNodes

			if nodes == nil {
				var err error

				if nodes, err = parseESI(testCase.Input); err != nil {
					t.Fatalf("failed to parse test input: %s", err)
				}
			}

			defaultOpts := []esiproc.ProcessorOpt{
				esiproc.WithIncludeConcurrency(4),
				esiproc.WithIncludeFunc(func(_ context.Context, urlStr string, meta map[string]string) ([]byte, error) {
					parsed, err := url.Parse(urlStr)
					if err != nil {
						panic(err)
					}

					switch parsed.Path {
					case "/error":
						return nil, errInvalid
					case "/panic":
						panic("unexpected include call")
					default:
						m := map[string]any{"url": urlStr}

						if meta != nil {
							m["meta"] = meta
						}

						return json.Marshal(m)
					}
				}),
				esiproc.WithEnv(testEnv{}),
			}

			p := esiproc.New(append(defaultOpts, testCase.Opts...)...)
			defer p.Release()

			var buf bytes.Buffer

			if err := p.Process(t.Context(), &buf, nodes); !errors.Is(err, testCase.Error) {
				t.Errorf("got error %v, want %v", err, testCase.Error)
			}

			if testCase.Error != nil {
				return
			}

			if diff := cmp.Diff(strings.TrimSpace(testCase.Expected), strings.TrimSpace(buf.String())); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
