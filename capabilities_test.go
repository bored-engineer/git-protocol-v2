package protocolv2

import "testing"

func TestCapability(t *testing.T) {
	tests := map[string]struct {
		input   string
		want    Capability
		wantErr string
	}{
		"invalid": {
			input:   "invalid",
			wantErr: "invalid capability: \"invalid\"",
		},
		"key": {
			input: "key\n",
			want:  Capability{Key: "key"},
		},
		"key=value": {
			input: "key=value\n",
			want:  Capability{Key: "key", Value: "value"},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var c Capability
			err := c.Parse([]byte(tc.input))
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error, got nil")
				} else if err.Error() != tc.wantErr {
					t.Fatalf("expected error %q, got %q", tc.wantErr, err)
				}
			}
			if c != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, c)
			}
		})
	}
}
