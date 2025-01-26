package protocolv2

import "testing"

func TestCommandArgument(t *testing.T) {
	tests := map[string]struct {
		input   string
		want    CommandArgument
		wantErr string
	}{
		"invalid": {
			input:   " value",
			wantErr: "invalid argument: \" value\"",
		},
		"key": {
			input: "key",
			want:  CommandArgument{Key: "key"},
		},
		"key value": {
			input: "key value",
			want:  CommandArgument{Key: "key", Value: "value"},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var ca CommandArgument
			err := ca.Parse([]byte(tc.input))
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error, got nil")
				} else if err.Error() != tc.wantErr {
					t.Fatalf("expected error %q, got %q", tc.wantErr, err)
				}
			}
			if ca != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, ca)
			}
		})
	}
}
