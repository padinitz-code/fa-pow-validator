package server

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strings"

	"github.com/rs/zerolog"
)

type BookQuote struct {
	Quote  string `json:"quote"`
	Author string `json:"author"`
}

type Book struct {
	quotes []*BookQuote
}

func NewBook(jsonQuotes []byte) (*Book, error) {
	var quotes []*BookQuote
	if err := json.Unmarshal(jsonQuotes, &quotes); err != nil {
		return nil, fmt.Errorf("unmarshal quotes: %w", err)
	}
	return &Book{quotes: quotes}, nil
}

func (b *Book) GetRandQuote() *BookQuote {
	i := rand.Intn(len(b.quotes))
	return b.quotes[i]
}

func (b *Book) ServeRequest(conn net.Conn, requestLog zerolog.Logger) {
	requestLog.Info().Msg("write response")

	q := b.GetRandQuote()
	r := strings.NewReader(q.Quote)
	_, err := io.Copy(conn, r)
	if err != nil {
		requestLog.Warn().Err(fmt.Errorf("io.Copy quote: %w", err)).Msg("failed to write response")
	}
}
