package core

import "testing"

func TestParseDocument(t *testing.T) {
	document := `
<!DOCTYPE html>
<html>
    <head>
        <title lang="xxx">==title==
</title>
        <style> css </style>
    </head>
    <body>
        <div> text1 <span>text2</span> text3  </div>
        <script> js code </script>
    </body>
</html>
`
	exceptedTitle := "==title=="
	exceptedBody := "text1 text2 text3"
	pd := parseDocument(document)
	if pd.title != exceptedTitle || pd.body != exceptedBody {
		t.Error("|"+pd.title+"|", "\n", "|"+pd.body+"|")
	}
}
