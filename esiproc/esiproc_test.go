package esiproc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"iter"
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

func nodesToSeq(nodes []esi.Node) iter.Seq2[esi.Node, error] {
	return func(yield func(esi.Node, error) bool) {
		for _, node := range nodes {
			if !yield(node, nil) {
				return
			}
		}
	}
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
				Element: &esi.ChooseElement{Position: esi.Position{Start: 5, End: 119}},
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
			Name:     "include with extra attributes",
			Input:    `before <esi:include src="/test" alt="/panic" attr1="value1" ns:attr2="value2"/> after`,
			Expected: `before {"url":"/test"} after`,
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
			defaultOpts := []esiproc.ProcessorOpt{
				esiproc.WithIncludeConcurrency(4),
				esiproc.WithIncludeFunc(func(_ context.Context, _ *esiproc.Processor, urlStr string) ([]byte, error) {
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
						return json.Marshal(map[string]any{"url": urlStr})
					}
				}),
				esiproc.WithEnv(testEnv{}),
			}

			p := esiproc.New(append(defaultOpts, testCase.Opts...)...)

			var nodes iter.Seq2[esi.Node, error]

			if testCase.InputNodes != nil {
				nodes = nodesToSeq(testCase.InputNodes)
			} else {
				nodes = esi.NewParser(strings.NewReader(testCase.Input)).All
			}

			var buf bytes.Buffer

			if _, err := p.Process(t.Context(), &buf, nodes); !errors.Is(err, testCase.Error) {
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

func BenchmarkProcessor(b *testing.B) {
	b.Run("Multiple includes", func(b *testing.B) {
		const input = `
			before request1 / <esi:include src="/request1"/> / after request1
			before request2 / <esi:include src="/request2"/> / after request2
			before request3 / <esi:include src="/request3"/> / after request3
			before request4 / <esi:include src="/request4"/> / after request4
			before request5 / <esi:include src="/request5"/> / after request5
			before request6 / <esi:include src="/request6"/> / after request6
			before request7 / <esi:include src="/request7"/> / after request7
			before request8 / <esi:include src="/request8"/> / after request8
			before request9 / <esi:include src="/request9"/> / after request9
			before request10 / <esi:include src="/request10"/> / after request10
		`

		var nodes []esi.Node

		for node, err := range esi.NewParser(strings.NewReader(input)).All {
			if err != nil {
				b.Fatal(err)
			}

			nodes = append(nodes, node)
		}

		p := esiproc.New(
			esiproc.WithIncludeConcurrency(4),
			esiproc.WithIncludeFunc(func(_ context.Context, _ *esiproc.Processor, urlStr string) ([]byte, error) {
				return []byte(urlStr), nil
			}))

		b.ReportAllocs()
		b.ResetTimer()

		for b.Loop() {
			if _, err := p.Process(b.Context(), io.Discard, nodesToSeq(nodes)); err != nil {
				b.Fatal(err)
			}
		}
	})
}
