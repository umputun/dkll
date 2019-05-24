package server

import (
	"testing"
)

func TestRest_MakeQuery(t *testing.T) {
	// r := RestServer{}
	// id := bson.NewObjectId()
	// req := Request{
	// 	Containers: []string{"cnt1", "cnt2"},
	// 	Hosts:      []string{"h1", "h2", "h3"},
	// 	Max:        100,
	// 	Excludes:   []string{"ex1", "ex2"},
	// 	FromTS:     "20180103:162517",
	// 	ToTS:       "20180105:195005",
	// }
	//
	// b, err := r.makeQuery(id, req)
	// require.NoError(t, err)
	// assert.Equal(t, bson.M{"$in": []interface{}{"cnt1", "cnt2"}, "$nin": []interface{}{"ex1", "ex2"}}, b["container"])
	// assert.Equal(t, bson.M{"$in": []interface{}{"h1", "h2", "h3"}}, b["host"])
	// assert.Equal(t, bson.M{"$gte": time.Date(2018, 1, 3, 16, 25, 17, 0, timeutils.NYLocation()),
	// 	"$lt": time.Date(2018, 1, 5, 19, 50, 5, 0, timeutils.NYLocation())}, b["ts"])
}
