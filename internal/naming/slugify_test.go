package naming

import (
	"testing"
)

func TestSlugify_EmptyInput(t *testing.T) {
	opts := SlugifyOptions{
		Lowercase:          true,
		ReplaceNonAlphaNum: true,
		CollapseDashes:     true,
		TrimDashes:         true,
	}

	result := Slugify("", opts)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestSlugify_Lowercase(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		lowercase bool
		want      string
	}{
		{"lowercase enabled", "Hello World", true, "hello world"},
		{"lowercase disabled", "Hello World", false, "Hello World"},
		{"already lowercase", "hello world", true, "hello world"},
		{"mixed case", "HeLLo WoRLD", true, "hello world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := SlugifyOptions{Lowercase: tt.lowercase}
			result := Slugify(tt.input, opts)
			if result != tt.want {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, result, tt.want)
			}
		})
	}
}

func TestSlugify_ReplaceNonAlphaNum(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		replace bool
		want    string
	}{
		{"replace enabled", "hello world!", true, "hello-world-"},
		{"replace disabled", "hello world!", false, "hello world!"},
		{"multiple special chars", "a@b#c$d", true, "a-b-c-d"},
		{"consecutive special chars", "hello...world", true, "hello-world"},
		{"only alphanumeric", "hello123", true, "hello123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := SlugifyOptions{ReplaceNonAlphaNum: tt.replace}
			result := Slugify(tt.input, opts)
			if result != tt.want {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, result, tt.want)
			}
		})
	}
}

func TestSlugify_CollapseDashes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		collapse bool
		want     string
	}{
		{"collapse enabled", "hello--world", true, "hello-world"},
		{"collapse disabled", "hello--world", false, "hello--world"},
		{"multiple dashes", "a---b----c", true, "a-b-c"},
		{"single dashes", "a-b-c", true, "a-b-c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := SlugifyOptions{CollapseDashes: tt.collapse}
			result := Slugify(tt.input, opts)
			if result != tt.want {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, result, tt.want)
			}
		})
	}
}

func TestSlugify_TrimDashes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		trim  bool
		want  string
	}{
		{"trim enabled", "-hello-world-", true, "hello-world"},
		{"trim disabled", "-hello-world-", false, "-hello-world-"},
		{"leading dashes", "---hello", true, "hello"},
		{"trailing dashes", "hello---", true, "hello"},
		{"no dashes to trim", "hello-world", true, "hello-world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := SlugifyOptions{TrimDashes: tt.trim}
			result := Slugify(tt.input, opts)
			if result != tt.want {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, result, tt.want)
			}
		})
	}
}

func TestSlugify_AllTransformations(t *testing.T) {
	opts := SlugifyOptions{
		Lowercase:          true,
		ReplaceNonAlphaNum: true,
		CollapseDashes:     true,
		TrimDashes:         true,
	}

	tests := []struct {
		input string
		want  string
	}{
		{"Hello World!", "hello-world"},
		{"  Multiple   Spaces  ", "multiple-spaces"},
		{"CamelCase", "camelcase"},
		{"with-dashes-already", "with-dashes-already"},
		{"123 Numbers 456", "123-numbers-456"},
		{"Special @#$ Characters", "special-characters"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := Slugify(tt.input, opts)
			if result != tt.want {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, result, tt.want)
			}
		})
	}
}

func TestSlugify_LengthLimitWithHash(t *testing.T) {
	opts := SlugifyOptions{
		MaxLength:          15,
		HashLength:         4,
		Lowercase:          true,
		ReplaceNonAlphaNum: true,
		CollapseDashes:     true,
		TrimDashes:         true,
	}

	// Input that exceeds max length
	input := "this is a very long string that needs truncation"
	result := Slugify(input, opts)

	if len(result) > opts.MaxLength {
		t.Errorf("result length %d exceeds max length %d", len(result), opts.MaxLength)
	}

	// Should contain a dash separator
	if result[len(result)-5] != '-' {
		t.Errorf("expected dash separator before hash, got %q", result)
	}
}

func TestSlugify_Deterministic(t *testing.T) {
	opts := SlugifyOptions{
		MaxLength:          10,
		HashLength:         4,
		Lowercase:          true,
		ReplaceNonAlphaNum: true,
		CollapseDashes:     true,
		TrimDashes:         true,
	}

	input := "some long input string"
	result1 := Slugify(input, opts)
	result2 := Slugify(input, opts)

	if result1 != result2 {
		t.Errorf("Slugify is not deterministic: %q != %q", result1, result2)
	}
}

func TestSlugify_DifferentInputsDifferentHashes(t *testing.T) {
	opts := SlugifyOptions{
		MaxLength:          10,
		HashLength:         4,
		Lowercase:          true,
		ReplaceNonAlphaNum: true,
		CollapseDashes:     true,
		TrimDashes:         true,
	}

	// These would collide without hash
	input1 := "abcdef123456789"
	input2 := "abcdef987654321"

	result1 := Slugify(input1, opts)
	result2 := Slugify(input2, opts)

	if result1 == result2 {
		t.Errorf("different inputs produced same slug: %q", result1)
	}

	// Both should be within max length
	if len(result1) > opts.MaxLength {
		t.Errorf("result1 length %d exceeds max length %d", len(result1), opts.MaxLength)
	}
	if len(result2) > opts.MaxLength {
		t.Errorf("result2 length %d exceeds max length %d", len(result2), opts.MaxLength)
	}
}

func TestSlugify_HashOnOriginalInput(t *testing.T) {
	opts := SlugifyOptions{
		MaxLength:          10,
		HashLength:         4,
		Lowercase:          true,
		ReplaceNonAlphaNum: true,
		CollapseDashes:     true,
		TrimDashes:         true,
	}

	// These differ only in case, should produce different hashes
	// because hash is computed on original input
	input1 := "ABCDEFGHIJKLMNOP"
	input2 := "abcdefghijklmnop"

	result1 := Slugify(input1, opts)
	result2 := Slugify(input2, opts)

	if result1 == result2 {
		t.Errorf("hash should be computed on original input, but results are identical: %q", result1)
	}
}

func TestSlugify_ShortInputNoHash(t *testing.T) {
	opts := SlugifyOptions{
		MaxLength:          20,
		HashLength:         4,
		Lowercase:          true,
		ReplaceNonAlphaNum: true,
		CollapseDashes:     true,
		TrimDashes:         true,
	}

	input := "short"
	result := Slugify(input, opts)

	// Short input should not have hash appended
	if result != "short" {
		t.Errorf("short input should not have hash, got %q", result)
	}
}

func TestSlugify_ExactMaxLength(t *testing.T) {
	opts := SlugifyOptions{
		MaxLength:          10,
		HashLength:         4,
		Lowercase:          true,
		ReplaceNonAlphaNum: true,
		CollapseDashes:     true,
		TrimDashes:         true,
	}

	// Input that's exactly max length after transformation
	input := "abcdefghij" // 10 chars
	result := Slugify(input, opts)

	// Should not have hash since it's exactly max length, not exceeding
	if result != "abcdefghij" {
		t.Errorf("exact max length input should not have hash, got %q", result)
	}
}

func TestSlugify_VerySmallMaxLength(t *testing.T) {
	opts := SlugifyOptions{
		MaxLength:          3,
		HashLength:         4,
		Lowercase:          true,
		ReplaceNonAlphaNum: true,
		CollapseDashes:     true,
		TrimDashes:         true,
	}

	input := "this is a long string"
	result := Slugify(input, opts)

	// Max length is smaller than hash + separator, should return truncated hash
	if len(result) > opts.MaxLength {
		t.Errorf("result length %d exceeds max length %d", len(result), opts.MaxLength)
	}
}

func TestSlugify_AllSpecialCharacters(t *testing.T) {
	opts := SlugifyOptions{
		Lowercase:          true,
		ReplaceNonAlphaNum: true,
		CollapseDashes:     true,
		TrimDashes:         true,
	}

	input := "@#$%^&*()"
	result := Slugify(input, opts)

	// All special chars become dashes, then collapsed to one, then trimmed
	if result != "" {
		t.Errorf("all special chars should result in empty string, got %q", result)
	}
}

func TestSlugify_Unicode(t *testing.T) {
	opts := SlugifyOptions{
		Lowercase:          true,
		ReplaceNonAlphaNum: true,
		CollapseDashes:     true,
		TrimDashes:         true,
	}

	input := "hello世界"
	result := Slugify(input, opts)

	// Unicode chars are non-alphanumeric, should be replaced with dash
	if result != "hello" {
		t.Errorf("unicode should be replaced, got %q", result)
	}
}

func TestSlugify_PrefixTrailingDashTrimmed(t *testing.T) {
	opts := SlugifyOptions{
		MaxLength:          10,
		HashLength:         4,
		Lowercase:          true,
		ReplaceNonAlphaNum: true,
		CollapseDashes:     true,
		TrimDashes:         false, // Note: TrimDashes is OFF
	}

	// This will become "hello-world-..." after transformations
	// When truncated, prefix might end with dash
	input := "hello world foo bar"
	result := Slugify(input, opts)

	// Should not have double dash even though TrimDashes is off
	// because we always trim trailing dash from prefix before adding separator
	if len(result) <= 5 {
		t.Skip("result too short to verify")
	}

	// Check there's no double dash before the hash
	for i := 0; i < len(result)-1; i++ {
		if result[i] == '-' && result[i+1] == '-' {
			t.Errorf("found double dash in result: %q", result)
		}
	}
}

func TestSlugify_NoOptions(t *testing.T) {
	opts := SlugifyOptions{}

	input := "Hello World!"
	result := Slugify(input, opts)

	// With no options, input should pass through unchanged
	if result != "Hello World!" {
		t.Errorf("with no options, expected unchanged input, got %q", result)
	}
}

func TestComputeHash(t *testing.T) {
	// Test hash is deterministic
	hash1 := computeHash("test", 4)
	hash2 := computeHash("test", 4)
	if hash1 != hash2 {
		t.Errorf("hash is not deterministic: %q != %q", hash1, hash2)
	}

	// Test different inputs produce different hashes
	hash3 := computeHash("test1", 4)
	hash4 := computeHash("test2", 4)
	if hash3 == hash4 {
		t.Errorf("different inputs produced same hash: %q", hash3)
	}

	// Test hash length
	hash5 := computeHash("test", 6)
	if len(hash5) != 6 {
		t.Errorf("expected hash length 6, got %d", len(hash5))
	}

	// Test zero length returns empty
	hash6 := computeHash("test", 0)
	if hash6 != "" {
		t.Errorf("expected empty hash for length 0, got %q", hash6)
	}

	// Test hash only contains valid base36 chars
	hash7 := computeHash("test string", 10)
	for _, c := range hash7 {
		if (c < '0' || c > '9') && (c < 'a' || c > 'z') {
			t.Errorf("hash contains invalid character: %c", c)
		}
	}
}
