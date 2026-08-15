package shared

import "testing"

func FuzzRoomCodeValidation(f *testing.F) {
	f.Add("FROG")
	f.Add("frog")
	f.Add("")
	f.Add("FROGS")
	f.Add("AB!C")
	f.Add("A\u00e9BC")
	f.Add(stringsRepeatA(300))

	f.Fuzz(func(t *testing.T, code string) {
		valid := IsValidRoomCode(code)

		if valid && len(code) != RoomCodeLength {
			t.Fatalf("IsValidRoomCode(%q) = true but length is %d", code, len(code))
		}

		// A valid code is already normalized.
		if valid && NormalizeRoomCode(code) != code {
			t.Fatalf("valid code %q is not normalized", code)
		}
	})
}

func stringsRepeatA(n int) string {
	b := make([]byte, n)

	for i := range b {
		b[i] = 'A'
	}

	return string(b)
}
