## Introduction:
This package provides functions for returning data to HTTP API calls in a consistent manner so clients can comprehend the data more easily.

## Details:
- Returns data as JSON.
- Each object returned has a status, a message type, the arbitrary data being returned, and a timestamp.
- Can be used for successful and error states.
- Message types allow for clients to understand various data coming from one endpoint.

## Data Format:
- OK: boolean.
- Type: string.
- Data: interface, can be anything. Structs are encoded as JS objects.
- Datetime: YYYY-MM-DD HH:MM:SS.sss string in UTC timezone.
- ErrorData: object with Error and Message fields.

## Message Types:
There are predefined message types included in the package that are used with the defined helper funcs (`InsertOK`, `DataFound`, `Error`, etc.). However, you can define your own message types and use them with `Success`. 

It is strongly advised to limit your custom message types, and keep track of them well, to reduce the inclination to "just use a new message type" with each response. This is more important in larger projects where different authors may create their own, sometimes overlapping, message types.

## Use:
This package was designed to send back data from a webapp to client-side JS code that interacts and renders the GUI. Having a consistent format was also nice for logging and diagnostics.

In most cases, you will use the higher level functions (`InsertOK`, `UpdateOK`, etc.) instead of `Success` as these functions will set a message type automatically and reduce the amount of code typed in your projects.

Some demo code:
```golang
//success
mt := output.MessageType("myCustomMessagType")
data := map[string]string{
    "key": "value",
    "wyle": "e coyote",
}
output.Success(mt, data, w)

//error
err := funcThatCausesError()
output.Error(err, "Human readable error message or how to fix the error.", w)
```
