# 0chain/errors

A simple errors package

```
go get github.com/0chain/errors
```

we introduce a new application error which has errorCode and errorMsg. 

```
type Error struct {
	Code string `json:"code,omitempty"`
	Msg  string `json:"msg"`
}
```

## New Error

Then `errors.New` function returns a new error given the code (optional) and msg

two arguments can be passed!
1. code
2. message
if only one argument is passed its considered as message
if two arguments are passed then
	first argument is considered for code and
	second argument is considered for message

```
 applicationError := errors.New("401", "Unauthorized")
 simpleError = errors.New("validation failed")
```


## Standard error interface implementation

This is what is printed when you do `.Error()` for the above example

```
fmt.Println(auth("username", "password"))

401: Unauthorized
validation failed
password mismatch
```

## Error propagation

The `errors.Wrap` function returns a new error that adds context to the original error. You can wrap using a msg or error. For example
```
var ErrPasswordMismatch = errors.New("password mismatch") // "invalid argument"
var ErrUnAuthorized = errors.New("401", "Unauthorized")

func auth(username, password string) error {
    err := validate(username, password)
    if err != nil {
        return errors.Wrap(err, ErrUnAuthorized)
    }
}


func validate(username, password string) error {
    err := passwordValidation(password)
    if err != nil {
        return errors.Wrap(err, "validation failed")
    }
}

func passwordValidation(password string) error{
    // on invalid password
    return ErrPasswordMismatch
}

```

The `errors.UnWrap` function returns the current error and the previous error

```
current, previous := errors.UnWrap(auth("username", "password"))

fmt.Println(current) => 401: Unauthorized
fmt.Println(previous) => validation failed
                         password mismatch

// futher more

current, previous := errors.UnWrap(previous)

fmt.Println(current) => validation failed
fmt.Println(previous) => password mismatch

// further more

current, previous := errors.UnWrap(previous)

fmt.Println(current) => password mismatch
fmt.Println(previous) => nil
```

For retrieving the cause of an error, The `errors.Cause` function is the way to go

```
err = auth("username", "password")

fmt.Println(errors.Cause(err)) => password mismatch
```

## Working with [Errors](https://blog.golang.org/go1.13-errors) package

How to raise an `ApplicationError` with predefined error variables?

```
var ErrInvalidFormat = errors.New("[conf]invalid format")

func ReadConfig()  (*Config,error) {
   //...
   return nil, Throw(ErrInvalidFormat, cfgFile)

}

func main() {

cfg, err := ReadConfig() 

if errors.Is(err, ErrInvalidFormat) {
    panic(err)
}

}

```

See [Unit Tests](throw_test.go) for more examples.


## Logging and track Unhandled Exception with traceid

### What is an Unhandled Exception?
An exception is a known type of error. An unhandled exception occurs when the application code does not properly handle exceptions. 

For example, When you try to read data from database, it is a common problem for the network is lost. We need show user an firendly message (eg. ServiceUnavailable),logging raw error in logging system, and trigger DevOps alert from log monitor system.


```
var ErrServiceUnavailable = errors.New("Service unavailable")
if err != nil { //any network/db error
    if errors.Is(err, ErrHasNotShared) {
         return nil, err
    }
    return nil, errors.ThrowLog(err,ErrServiceUnavailable)
}
```

