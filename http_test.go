package healthzhttp

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cheekybits/is"
	"github.com/gorilla/mux"
	"github.com/jasonhancock/healthz"
)

func TestCheckHTTP(t *testing.T) {
	is := is.New(t)

	status := http.StatusOK
	allowedMethod := "ALL"

	handle := func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if allowedMethod != "ALL" && r.Method != allowedMethod {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		w.WriteHeader(status)
		w.Write(body)
	}

	router := mux.NewRouter()
	router.HandleFunc("/echo", handle).Methods(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete)
	server := httptest.NewServer(router)
	defer server.Close()

	u := server.URL + "/echo"

	// A plain check...should test for a 200
	c, err := NewCheck(u)
	is.NoErr(err)
	result := c.Check(context.Background())
	is.NoErr(result.Error)

	// set the status to a 404, we should get an error
	status = http.StatusNotFound
	result = c.Check(context.Background())
	is.Err(result.Error)
	is.True(strings.HasPrefix(result.Error.Error(), "Unexpected http status code:"))

	c.allowedStatusCodes[http.StatusNotFound] = struct{}{}
	result = c.Check(context.Background())
	is.NoErr(result.Error)
	delete(c.allowedStatusCodes, http.StatusNotFound)

	// change to a different method
	status = http.StatusOK
	allowedMethod = http.MethodPost
	result = c.Check(context.Background())
	is.Err(result.Error)
	is.True(strings.HasPrefix(result.Error.Error(), "Unexpected http status code:"))

	// Update the expected method
	c.method = http.MethodPost
	result = c.Check(context.Background())
	is.NoErr(result.Error)
}

func TestAllowedStatusCodes(t *testing.T) {
	is := is.New(t)
	c, err := NewCheck("http://example.com/healthz", WithoutAllowedStatusCode(200), WithAllowedStatusCode(302))
	is.NoErr(err)
	_, ok := c.allowedStatusCodes[200]
	is.False(ok)
	_, ok = c.allowedStatusCodes[302]
	is.OK(ok)
}

func ExampleCheckHTTP() {
	checker := healthz.NewChecker()
	chk, err := NewCheck("http://example.com/path")
	if err != nil {
		log.Fatal(err)
	}
	checker.AddCheck("remote_service", chk)
	http.ListenAndServe(":8080", checker)
}
