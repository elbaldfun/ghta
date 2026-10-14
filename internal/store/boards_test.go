package store

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func TestBoards(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	type row struct {
		ID string `bson:"id"`
		N  int    `bson:"n"`
	}
	var got []row
	if _, err := st.Board(ctx, "k", &got); err != ErrNoBoard {
		t.Fatalf("missing board: %v; want ErrNoBoard", err)
	}
	day1 := time.Date(2026, 10, 14, 4, 0, 0, 0, time.UTC)
	if err := st.PutBoard(ctx, "k", []row{{"a", 1}, {"b", 2}}, day1); err != nil {
		t.Fatal(err)
	}
	// A board is replaced whole.
	if err := st.PutBoard(ctx, "k", []row{{"c", 3}}, day1.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	at, err := st.Board(ctx, "k", &got)
	if err != nil || !at.Equal(day1.Add(time.Hour)) || len(got) != 1 || got[0] != (row{"c", 3}) {
		t.Fatalf("board = %+v at %v, %v", got, at, err)
	}

	if err := st.PutBoardHead(ctx, "k", "2026-10-14", []string{"a", "b"}, day1); err != nil {
		t.Fatal(err)
	}
	if err := st.PutBoardHead(ctx, "k", "2026-10-14", []string{"b", "a"}, day1); err != nil {
		t.Fatal(err)
	}
	ids, err := st.BoardHead(ctx, "k", "2026-10-14")
	if err != nil || len(ids) != 2 || ids[0] != "b" {
		t.Fatalf("head = %v, %v; want the rerun's order", ids, err)
	}
	if ids, _ := st.BoardHead(ctx, "k", "2026-10-13"); ids != nil {
		t.Fatalf("head of a day not recorded = %v", ids)
	}
	if n := count(t, st, CollBoardsHistory, bson.M{}); n != 1 {
		t.Fatalf("history docs = %d; a day has one head", n)
	}
}
