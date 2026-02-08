# TODO: Compile Verification Tests

Issue: Add tests that verify generated code compiles correctly for each target.

## Go Target
- [ ] Generate Go code and run `go build` on output
- [ ] Test with `go vet` for additional validation
- [ ] Can use temp directory in test

## Proto Target
- [ ] Generate .proto file and run `protoc --lint_out=.` or similar
- [ ] Verify proto3 syntax is valid
- [ ] Consider `buf lint` for modern proto linting

## Implementation Approach

```go
// Example test structure:
func TestGeneratedGoCompiles(t *testing.T) {
    // Generate code
    output := generateForModel(testModel)
    
    // Write to temp dir
    tmpDir := t.TempDir()
    writeFiles(tmpDir, output)
    
    // Run go build
    cmd := exec.Command("go", "build", "./...")
    cmd.Dir = tmpDir
    if err := cmd.Run(); err != nil {
        t.Fatalf("generated code failed to compile: %v", err)
    }
}
```

## Priority
Medium - validates that generated code is actually usable, not just syntactically correct.
