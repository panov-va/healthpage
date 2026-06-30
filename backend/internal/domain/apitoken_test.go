package domain

import "testing"

func TestTokenScopeIsValid(t *testing.T) {
	for _, s := range AllTokenScopes {
		if !s.IsValid() {
			t.Errorf("%q должен быть валиден", s)
		}
	}
	for _, s := range []TokenScope{"", "admin", "READ", "delete"} {
		if s.IsValid() {
			t.Errorf("%q не должен быть валиден", s)
		}
	}
}

func TestAPITokenHasScope(t *testing.T) {
	tests := []struct {
		name      string
		scopes    []TokenScope
		wantRead  bool
		wantWrite bool
	}{
		{"write подразумевает read", []TokenScope{ScopeWrite}, true, true},
		{"только read", []TokenScope{ScopeRead}, true, false},
		{"оба", []TokenScope{ScopeRead, ScopeWrite}, true, true},
		{"пусто", nil, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tok := APIToken{Scopes: tc.scopes}
			if got := tok.HasScope(ScopeRead); got != tc.wantRead {
				t.Errorf("HasScope(read) = %v, want %v", got, tc.wantRead)
			}
			if got := tok.HasScope(ScopeWrite); got != tc.wantWrite {
				t.Errorf("HasScope(write) = %v, want %v", got, tc.wantWrite)
			}
			if got := tok.CanWrite(); got != tc.wantWrite {
				t.Errorf("CanWrite() = %v, want %v", got, tc.wantWrite)
			}
		})
	}
}

func TestNormalizeScopes(t *testing.T) {
	got, ok := NormalizeScopes([]string{"read", "write", "read"})
	if !ok {
		t.Fatal("валидный набор должен пройти")
	}
	if len(got) != 2 || got[0] != ScopeRead || got[1] != ScopeWrite {
		t.Errorf("дедупликация/порядок неверны: %v", got)
	}

	if got, ok := NormalizeScopes(nil); !ok || len(got) != 0 {
		t.Errorf("пустой набор: ok=%v len=%d", ok, len(got))
	}

	if _, ok := NormalizeScopes([]string{"read", "admin"}); ok {
		t.Error("недопустимый scope должен дать ok=false")
	}
}
