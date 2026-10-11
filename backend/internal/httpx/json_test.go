package httpx

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

type trimInner struct {
	City string `json:"city"`
}

type trimTarget struct {
	Name     string            `json:"name"`
	Email    *string           `json:"email"`
	Inner    trimInner         `json:"inner"`
	PtrInner *trimInner        `json:"ptr_inner"`
	Tags     []string          `json:"tags"`
	Items    []trimInner       `json:"items"`
	Labels   map[string]string `json:"labels"`
	Password string            `json:"password" trim:"-"`
	Count    int               `json:"count"`
	Ratio    float64           `json:"ratio"`
	Flag     bool              `json:"flag"`
	hidden   string
}

func jsonRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	return r
}

func TestReadJSONTrimsEveryStringInTheBody(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		check func(t *testing.T, got *trimTarget)
	}{
		{
			name: "top-level string",
			body: `{"name":"  Afa \t"}`,
			check: func(t *testing.T, got *trimTarget) {
				if got.Name != "Afa" {
					t.Errorf("name: want %q; got %q", "Afa", got.Name)
				}
			},
		},
		{
			name: "nested struct",
			body: `{"inner":{"city":"\n Rosario  "}}`,
			check: func(t *testing.T, got *trimTarget) {
				if got.Inner.City != "Rosario" {
					t.Errorf("inner.city: want %q; got %q", "Rosario", got.Inner.City)
				}
			},
		},
		{
			name: "pointer to string",
			body: `{"email":"  a@b.com  "}`,
			check: func(t *testing.T, got *trimTarget) {
				if got.Email == nil || *got.Email != "a@b.com" {
					t.Errorf("email: want %q; got %v", "a@b.com", got.Email)
				}
			},
		},
		{
			name: "absent pointers stay nil",
			body: `{"name":"x"}`,
			check: func(t *testing.T, got *trimTarget) {
				if got.Email != nil {
					t.Errorf("email: want nil; got %q", *got.Email)
				}
				if got.PtrInner != nil {
					t.Errorf("ptr_inner: want nil; got %+v", *got.PtrInner)
				}
			},
		},
		{
			name: "pointer to struct",
			body: `{"ptr_inner":{"city":" CABA "}}`,
			check: func(t *testing.T, got *trimTarget) {
				if got.PtrInner == nil || got.PtrInner.City != "CABA" {
					t.Errorf("ptr_inner.city: want %q; got %+v", "CABA", got.PtrInner)
				}
			},
		},
		{
			name: "slice of strings",
			body: `{"tags":[" pileta ","   ","quincho"]}`,
			check: func(t *testing.T, got *trimTarget) {
				want := []string{"pileta", "", "quincho"}
				if !reflect.DeepEqual(got.Tags, want) {
					t.Errorf("tags: want %q; got %q", want, got.Tags)
				}
			},
		},
		{
			name: "slice of structs",
			body: `{"items":[{"city":" a "},{"city":"b "}]}`,
			check: func(t *testing.T, got *trimTarget) {
				if len(got.Items) != 2 || got.Items[0].City != "a" || got.Items[1].City != "b" {
					t.Errorf("items: want cities a, b; got %+v", got.Items)
				}
			},
		},
		{
			name: "map values",
			body: `{"labels":{"k":"  v  "}}`,
			check: func(t *testing.T, got *trimTarget) {
				if got.Labels["k"] != "v" {
					t.Errorf("labels[k]: want %q; got %q", "v", got.Labels["k"])
				}
			},
		},
		{
			name: "trim dash tag keeps the value as sent",
			body: `{"password":"  s3cret  "}`,
			check: func(t *testing.T, got *trimTarget) {
				if got.Password != "  s3cret  " {
					t.Errorf("password: want it untouched; got %q", got.Password)
				}
			},
		},
		{
			name: "non-string fields are untouched",
			body: `{"count":3,"ratio":1.5,"flag":true}`,
			check: func(t *testing.T, got *trimTarget) {
				if got.Count != 3 || got.Ratio != 1.5 || !got.Flag {
					t.Errorf("want count=3 ratio=1.5 flag=true; got %d %v %t", got.Count, got.Ratio, got.Flag)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got trimTarget

			err := ReadJSON(httptest.NewRecorder(), jsonRequest(t, tt.body), &got)
			if err != nil {
				t.Fatalf("ReadJSON: %v", err)
			}
			tt.check(t, &got)
		})
	}
}

func TestReadJSONLeavesUnexportedFieldsAlone(t *testing.T) {
	got := trimTarget{hidden: "  kept  "}

	err := ReadJSON(httptest.NewRecorder(), jsonRequest(t, `{"name":" a "}`), &got)
	if err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if got.hidden != "  kept  " {
		t.Errorf("unexported field: want it untouched; got %q", got.hidden)
	}
}

// ReadJSONAnyContentType serves third-party payloads and must stay byte-faithful.
func TestReadJSONAnyContentTypeDoesNotTrim(t *testing.T) {
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{"name":"  Afa  ","password":" p "}`))

	var got trimTarget
	err := ReadJSONAnyContentType(httptest.NewRecorder(), r, &got)
	if err != nil {
		t.Fatalf("ReadJSONAnyContentType: %v", err)
	}
	if got.Name != "  Afa  " {
		t.Errorf("name: want the value as sent; got %q", got.Name)
	}
}
