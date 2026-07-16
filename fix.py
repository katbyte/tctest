import os

with open('cli/pr-ast.go', 'r') as f:
    content = f.read()

# Fix TraceHelperFiles
content = content.replace("pf := provider.NewFileWithPath(f, dc.RepoPath)", "pf := f")

# allHelperTraced is map[string][]provider.File
content = content.replace("allHelperTraced[f] = append(allHelperTraced[f], tracedFile)", "allHelperTraced[f.RelPath] = append(allHelperTraced[f.RelPath], tracedFile)")
content = content.replace("traced := allHelperTraced[f]", "traced := allHelperTraced[f.RelPath]")

# filepath.Dir(f) when f is provider.File
content = content.replace("dir := filepath.ToSlash(filepath.Dir(f))", "dir := filepath.ToSlash(filepath.Dir(f.RelPath))")

with open('cli/pr-ast.go', 'w') as f:
    f.write(content)
