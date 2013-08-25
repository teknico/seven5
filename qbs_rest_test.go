package seven5

import (
	"github.com/coocood/qbs"
	_ "github.com/lib/pq"
	"net/http"
	"os"
	"testing"
)

/*---- type of actual DB action ----*/
type House struct {
	Address string
	Zip     int /*0->99999, inclusive*/
}

/*---- wire type for the tests ----*/
type HouseWire struct {
	Id Id
}

type testObj struct {
	testCallCount int
}

/*these funcs are use to test that if you meet the QBSRest interfaces you
can be wrapped by the qbs code in Seven5 */
func (self *testObj) Index(pb PBundle, q *qbs.Qbs) (interface{}, error) {
	self.testCallCount++
	return &HouseWire{}, nil
}
func (self *testObj) Find(id Id, pb PBundle, q *qbs.Qbs) (interface{}, error) {
	self.testCallCount++
	return &HouseWire{}, nil
}
func (self *testObj) Delete(id Id, pb PBundle, q *qbs.Qbs) (interface{}, error) {
	self.testCallCount++
	return &HouseWire{}, nil
}
func (self *testObj) Put(id Id, value interface{}, pb PBundle, q *qbs.Qbs) (interface{}, error) {
	self.testCallCount++
	return &HouseWire{}, nil
}
func (self *testObj) Post(value interface{}, pb PBundle, q *qbs.Qbs) (interface{}, error) {
	self.testCallCount++
	return &HouseWire{}, nil
}

/*-------------------------------------------------------------------------*/
/*                                 TEST CODE                               */
/*-------------------------------------------------------------------------*/
const (
	APP_NAME = "testapp"
)

func TestTxn(T *testing.T) {
}

func TestWrapping(T *testing.T) {
	os.Setenv("TESTAPP_DBNAME", "seven5test")
	env := NewEnvironmentVars(APP_NAME)

	io := NewRawIOHook(&JsonDecoder{}, &JsonEncoder{}, nil)
	raw := NewRawDispatcher(io, nil, nil, NewSimpleTypeHolder(), "/rest")

	obj := new(testObj)

	serveMux := NewServeMux()
	serveMux.Dispatch("/rest/", raw)

	store := NewQbsStore(env)

	raw.ResourceSeparate("house", &HouseWire{},
		QbsWrapIndex(obj, store),
		QbsWrapFind(obj, store),
		QbsWrapPost(obj, store),
		QbsWrapPut(obj, store),
		QbsWrapDelete(obj, store))
	if obj.testCallCount != 0 {
		T.Fatalf("sanity check at start failed: %d", obj.testCallCount)
	}

	go func() {
		http.ListenAndServe(":8991", serveMux)
	}()

	client := new(http.Client)

	messageData := [][]string{
		[]string{"GET", "http://localhost:8991/rest/house", ""},
		[]string{"GET", "http://localhost:8991/rest/house/1", ""},
		[]string{"DELETE", "http://localhost:8991/rest/house/1", ""},
		[]string{"POST", "http://localhost:8991/rest/house", "{}"},
		[]string{"PUT", "http://localhost:8991/rest/house/1", "{}"},
	}
	for i, callCount := range []int{1, 2, 3, 4, 5} {
		req := makeReq(T, messageData[i][0], messageData[i][1], messageData[i][2])
		resp, err := client.Do(req)
		checkResponse(T, err, resp)
		if obj.testCallCount != callCount {
			T.Errorf("did not call Hood level resource (expected %d calls but found %d)", callCount, obj.testCallCount)
		}
	}
}

func checkResponse(T *testing.T, err error, resp *http.Response) {
	if err != nil {
		T.Fatalf("failed on %s with error %v", "GET", err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		T.Fatalf("failed on %s with status %d", "GET", resp.StatusCode)
	}
}