package esiproc_test

import (
	"bytes"
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/nussjustin/esi"
	"github.com/nussjustin/esi/esiproc"
)

func TestProcessor(t *testing.T) {
	errInvalid := errors.New("invalid input")

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
			InputNodes: esi.Nodes{
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
				esiproc.WithTestFunc(nil),
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
			InputNodes: esi.Nodes{
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
			Input:    `before <esi:include src="/echo?string=src"/> after`,
			Expected: `before src after`,
		},
		{
			Name:     "include with alt",
			Input:    `before <esi:include src="/echo?string=src" alt="/panic"/> after`,
			Expected: `before src after`,
		},
		{
			Name:  "include error",
			Input: `before <esi:include src="/error"/> after`,
			Error: errInvalid,
		},
		{
			Name:     "include error with alt",
			Input:    `before <esi:include src="/error" alt="/echo?string=alt"/> after`,
			Expected: `before alt after`,
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
			Name: "include without fetch func",
			Opts: []esiproc.ProcessorOpt{
				esiproc.WithFetchFunc(nil),
			},
			Input: `before <esi:include src="/echo?string=included"/> after`,
			Error: &esiproc.UnsupportedElementError{
				Element: &esi.IncludeElement{Position: esi.Position{Start: 7, End: 49}},
			},
		},
		{
			Name: "otherwise outside choose",
			InputNodes: esi.Nodes{
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
					<esi:attempt><esi:include src="/echo?string=attempt"/></esi:attempt>
					<esi:except><esi:include src="/panic"/></esi:except>
				</esi:try>
			`,
			Expected: `attempt`,
		},
		{
			Name: "try with failed include",
			Input: `
				<esi:try>
					<esi:attempt><esi:include src="/error"/></esi:attempt>
					<esi:except><esi:include src="/echo?string=except"/></esi:except>
				</esi:try>
			`,
			Expected: `except`,
		},
		{
			Name: "try with failed include with alt",
			Input: `
				<esi:try>
					<esi:attempt><esi:include src="/error" alt="/echo?string=alt"/></esi:attempt>
					<esi:except><esi:include src="/panic"/></esi:except>
				</esi:try>
			`,
			Expected: `alt`,
		},
		{
			Name: "try with failed include with onerror=continue",
			Input: `
				<esi:try>
					<esi:attempt><esi:include src="/error" onerror="continue"/></esi:attempt>
					<esi:except><esi:include src="/echo?string=except"/></esi:except>
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
			InputNodes: esi.Nodes{
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
								<esi:include src="/error" alt="/echo?string=hello%20world"/>
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
								hello world
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

				if nodes, err = esi.Parse([]byte(testCase.Input)); err != nil {
					t.Fatalf("failed to parse test input: %s", err)
				}
			}

			defaultOpts := []esiproc.ProcessorOpt{
				esiproc.WithFetchConcurrency(4),
				esiproc.WithFetchFunc(func(_ context.Context, urlStr string) ([]byte, error) {
					parsed, err := url.Parse(urlStr)
					if err != nil {
						panic(err)
					}

					switch parsed.Path {
					case "/echo":
						return []byte(parsed.Query().Get("string")), nil
					case "/error":
						return nil, errInvalid
					default:
						panic("unexpected fetch call")
					}
				}),
				esiproc.WithTestFunc(func(_ context.Context, expr string) (bool, error) {
					switch expr {
					case "false":
						return false, nil
					case "true":
						return true, nil
					default:
						return false, errInvalid
					}
				}),
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
