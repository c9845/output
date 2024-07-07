/*
Package output is used to return data from requests to an HTTP API and present the
returned data in a consistent manner for ease of comprehension by clients.

The returned data has a strict format except for user-defined data stored in the
returned data's Data field. The returned data has a message type, a status, the
arbitrary user-defined data, and a timestamp.

The message type field, Type, is used by clients to understand the topic of the
returned data. Typically you would choose a message type from a predefined list
(either defined in this package or defined by you prior to use) to reduce the
possible message types a client would need to expect. Message types are short,
descriptive titles to the returned data.

The status field, OK, is a boolean field that simply states if an error occured while
processing the request.

The arbitrary data being returned is stored in one of two fields, depending on if an
error occured (and OK is false). We use two fields, instead of one, so that we can
store error data a bit differently (storing machine-readable error type and
human-readable error message), and so that clients don't check that OK is false and
assume a mistake since the Data field will be empty.

The main functions you will use are Success and Error, with the helper functions
InsertOK, UpdateOK, DataFound, being used in place of Success. The error returned
from these functions can usually be ignored; the error is only useful if you are
defining custom message types, EnforceStrictMessageTypes is enabled, and you used a
not-previously defined message type in the call to Success or its wrapper functions.
The error will report that you must use a defined message type.
*/
package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// Some message types are predefined due to common use.
const (
	msgTypeError     = "error"     //used when returning an error with the Error function.
	msgTypeInsertOK  = "insertOK"  //used when inserting into a database is successful with the InsertOK function.
	msgTypeUpdateOK  = "updateOK"  //used when updating a database is successful with the UpdateOK function.
	msgTypeDeleteOK  = "deleteOK"  //used when deleting something in the database is successful with the DeleteOK function.
	msgTypeDataFound = "dataFound" //used when retrieving data from the database is successful with the DataFound function.
)

// Define errors returned in HTTP responses.
var (
	errInputInvalid  = errors.New("input validation error")
	errAlreadyExists = errors.New("already exists")
)

// Define errors that can occur with our funcs that prevent writing responses and
// that the parent func calling of the of funcs in this package needs to handle.
var (
	//ErrInvalidResponseCode is returned when a non-existant HTTP status code is
	//provided.
	ErrInvalidResponseCode = errors.New("output: invalid HTTP response code")
)

// Payload is the format of the data that will be sent back to the requestor client.
// This format is designed so that data being returned to the client is always in a
// consistent format.
type Payload struct {
	//OK reports the overall status of a request.
	//
	//If OK is true, the request was completed successfully.
	//If OK is false, an error occured during handling of the request.
	OK bool

	//Type is a descriptive title for response data so that clients can understand
	//the data in the response. You would typically use one of the defined message
	//types, or one of the functions that sets a message type automatically, although
	//you can define your own custom message types as needed.
	Type string

	//Data is arbitrary data to send back to the client. This is the data from you
	//application.
	//
	//This field is typically only populated when OK is true, however, it can be
	//populated in rare circumstances when OK is false (see ErrorWithID).
	Data interface{} `json:",omitempty"`

	//ErrorData is the data returned when an error occurs. We use a different field
	//when returning error data to reduce confusion. A lower-level error type and a
	//human-readable error message are returned.
	//
	//This field is only populated when OK is false.
	ErrorData ErrorPayload `json:",omitempty"`

	//Datetime is simply a timestamp of when a mesage was created. This is typically
	//used for diagnostics on the client side. It is YYYY-MM-DD HH:MM:SS.sss
	//formatted in the UTC timezone.
	Datetime string
}

// ErrorPayload is descriptive data about an error.
type ErrorPayload struct {
	//Error is a lower-level error, typically an err returned from a func.
	Error string `json:",omitempty"`

	//Message is a higher-level, more human-friendly, message that can be displayed
	//in a GUI and explains how to resolve the error.
	Message string `json:",omitempty"`
}

// buildAndSend builds a Payload from the provided ok, msgType, msgData, and errData
// and then calls send().
func buildAndSend(ok bool, msgType string, msgData interface{}, errData ErrorPayload, w http.ResponseWriter, responseCode int) (err error) {
	//Get timestamp for response. This is used for diagnostics. The "Z" is appended
	//to the end to signify the datetime is in the UTC timezone.
	t := time.Now().UTC().Format("2006-01-02T15:04:05.000") + "Z"

	//Build data object being returned.
	//
	//Note that Data or ErrorData will be removed from JSON if they are empty (per
	//struct tags on fields).
	p := Payload{
		OK:        ok,
		Type:      msgType,
		Data:      msgData,
		ErrorData: errData,
		Datetime:  t,
	}

	//Send the response.
	err = p.send(w, responseCode)
	return
}

// send handles actually sending the response.
func (p *Payload) send(w http.ResponseWriter, responseCode int) (err error) {
	//Set the response code.
	w.WriteHeader(responseCode)

	//Set the content type.
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	//Send back the JSON response.
	j, err := json.Marshal(p)
	w.Write(j)
	return
}

// Send is used to send any response, with any payload, and any response code. This
// is meant to be used in situations where the Success and Error (and related helper
// funcs) do not provide enough control over the response, specifically when you want
// to use non-200 and -500 HTTP status codes.
func Send(p Payload, w http.ResponseWriter, responseCode int) (err error) {
	//Do some validation since the payload is constructed manually.
	if strings.TrimSpace(p.Datetime) == "" {
		p.Datetime = time.Now().UTC().Format("2006-01-02T15:04:05.000") + "Z"
	}

	//If ErrorData is provided, OK must be false. Data can still be provided when
	//errors occur though (see ErrorWithID()).
	if p.ErrorData != (ErrorPayload{}) {
		p.OK = false
	}

	//Make sure a response code was provided.
	if responseCode < http.StatusContinue {
		if debug {
			log.Println("output.Send", "invalid HTTP response code provided", responseCode)
		}

		err = ErrInvalidResponseCode
		return
	}

	//Make sure a message type was provided. Message type is used to handle responses
	//by the client so this field is required.
	//
	//If a type wasn't provided, use the HTTP status code. The code and status
	//message are concatted together so hopefully help developers identify that they
	//did not provide a status code manually.
	if strings.TrimSpace(p.Type) == "" {
		p.Type = fmt.Sprintf("%d-%s", responseCode, http.StatusText(responseCode))

		if debug {
			log.Println("output.Send", "payload has not message type, defaulting to type based on HTTP response code.", responseCode, p.Type)
		}
	}

	//We could do some checking if a 4xx code was provided if ErrorData was also
	//provided, but we would rather not enforce a set of status codes since each
	//application that uses this package may have different ideas of how to use
	//ErrorData and what applicable code to return.

	//Send the response.
	err = p.send(w, responseCode)
	return
}

// debug is used to enable diagnostic logging.
var debug = false

// Debug turns debug logging on or off.
func Debug(b bool) {
	debug = b
}

// Success is used when a request was successful and one of the other successful
// response funcs (InsertOK, UpdateOK, DataFound, etc.) doesn't fit. While an error
// is returned, it is typically ignored.
//
// Success, and related functions, always returns an HTTP status 200.
func Success(msgType string, data interface{}, w http.ResponseWriter) (err error) {
	err = buildAndSend(true, msgType, data, ErrorPayload{}, w, http.StatusOK)
	return
}

// InsertOK is used when a request resulted in data being successfully inserted into
// a database. This allows for sending by the just inserted data's ID.
func InsertOK(id int64, w http.ResponseWriter) (err error) {
	err = Success(msgTypeInsertOK, id, w)
	return
}

// InsertOKWithData is used when a request resulted in data being successfully
// inserted into a database and you want to send back a bunch of data with the
// response. While InsertOK can only send back an integer ID, this can send back
// anything.
func InsertOKWithData(data interface{}, w http.ResponseWriter) (err error) {
	err = Success(msgTypeInsertOK, data, w)
	return
}

// UpdateOK is used when a request resulted in data being successfully updated in a
// database.
func UpdateOK(w http.ResponseWriter) (err error) {
	err = Success(msgTypeUpdateOK, nil, w)
	return
}

// UpdateOKWithData is used when a request resulted in data being successfully
// updated in a database and you want to send back a bunch of data with the response.
func UpdateOKWithData(data interface{}, w http.ResponseWriter) (err error) {
	err = Success(msgTypeUpdateOK, data, w)
	return
}

// DataFound is used to send back data in a response. This is typically used with
// looking up data from a database.
func DataFound(data interface{}, w http.ResponseWriter) (err error) {
	err = Success(msgTypeDataFound, data, w)
	return
}

// Error is used when an error occured with a request and one of the other error
// response funcs (ErrorInputInvalid, etc.) doesn't fit.
//
// Error, and related functions, always returns an HTTP status 500.
func Error(errType error, errMsg string, w http.ResponseWriter) (err error) {
	//Define the error related data.
	ep := ErrorPayload{
		Error:   errType.Error(),
		Message: errMsg,
	}

	//Logging of errors can be used for diagnostics.
	if debug {
		log.Println("output.Error", errType, errMsg)
	}

	err = buildAndSend(false, msgTypeError, nil, ep, w, http.StatusInternalServerError)
	return
}

// ErrorInputInvalid is used when an error occurs while performing input validation.
func ErrorInputInvalid(msg string, w http.ResponseWriter) (err error) {
	err = Error(errInputInvalid, msg, w)
	return
}

// ErrorAlreadyExists is used when trying to insert something into the db that already
// exists.
func ErrorAlreadyExists(msg string, w http.ResponseWriter) (err error) {
	err = Error(errAlreadyExists, msg, w)
	return
}

// ErrorWithID is similar to Error but allows for returning an ID and the error data.
// This is used when you saved some data to a database and you want subsequent
// request to "retry" using the existing ID instead of recreating records over an
// over with each error.
func ErrorWithID(errType error, errMsg string, id int64, w http.ResponseWriter) (err error) {
	ep := ErrorPayload{
		Error:   errType.Error(),
		Message: errMsg,
	}

	if debug {
		log.Println("output.ErrorWithID", errType, errMsg, id)
	}

	err = buildAndSend(false, msgTypeError, id, ep, w, http.StatusInternalServerError)
	return
}

// ErrorInputInvalidWithID is similar to ErrorInputInvalid but allows for returning
// an I when an input validation error occured. This is used when you saved some data
// to a database and you want subsequent requests to "retry" using the existing ID
// instead of recreating records over an over with each error.
func ErrorInputInvalidWithID(msg string, id int64, w http.ResponseWriter) (err error) {
	err = ErrorWithID(errInputInvalid, msg, id, w)
	return
}
