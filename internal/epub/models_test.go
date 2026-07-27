package epub

import (
	"encoding/json"
	"testing"
)

func TestReadableArticleStringIsValidJSON(t *testing.T) {
	article := ReadableArticle{
		Title:   "A Title",
		Author:  "An Author",
		Content: "<p>body</p>",
	}

	var got ReadableArticle
	if err := json.Unmarshal([]byte(article.String()), &got); err != nil {
		t.Fatalf("String() produced invalid JSON: %v\n%s", err, article.String())
	}
	if got.Title != article.Title || got.Author != article.Author || got.Content != article.Content {
		t.Errorf("round-tripped article = %#v, want %#v", got, article)
	}
}
