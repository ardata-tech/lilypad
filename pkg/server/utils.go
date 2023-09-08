package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/bacalhau-project/lilypad/pkg/web3"
	"github.com/rs/zerolog/log"
)

// write some string constants for x-lilypad headers
// this is the address of the user
const X_LILYPAD_USER = "X-Lilypad-User"

// this is the signature of the message
const X_LILYPAD_SIGNATURE = "X-Lilypad-Signature"

// the context name we keep the address
const CONTEXT_ADDRESS = "address"

type HTTPError struct {
	Message    string
	StatusCode int
}

func (e HTTPError) Error() string {
	return e.Message
}

func extractUserAddress(userPayload string, signature string) (string, error) {
	address, err := web3.GetAddressFromSignedMessage([]byte(userPayload), []byte(signature))
	if err != nil {
		return "", err
	}
	return address.String(), nil
}

func setContextAddress(ctx context.Context, address string) context.Context {
	return context.WithValue(ctx, CONTEXT_ADDRESS, address)
}

func GetContextAddress(ctx context.Context) string {
	address, ok := ctx.Value(CONTEXT_ADDRESS).(string)
	if !ok {
		return ""
	}
	return address
}

// this will use the client headers to ensure that a message was signed
// by the holder of a private key for a specific address
// there is a "X-Lilypad-User" header that will contain the address
// there is a "X-Lilypad-Signature" header that will contain the signature
// we use the signature to verify that the message was signed by the private key
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		address, err := extractUserAddress(req.Header.Get(X_LILYPAD_USER), req.Header.Get(X_LILYPAD_SIGNATURE))
		if err != nil {
			http.Error(res, err.Error(), http.StatusForbidden)
			return
		}
		req = req.WithContext(setContextAddress(req.Context(), address))
		next.ServeHTTP(res, req)
	})
}

func CorsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Access-Control-Allow-Origin", "*")
		next.ServeHTTP(res, req)
	})
}

type httpWrapper[T any] func(res http.ResponseWriter, req *http.Request) (T, error)

func ReadBody[T any](req *http.Request) (T, error) {
	var data T
	err := json.NewDecoder(req.Body).Decode(&data)
	if err != nil {
		return data, err
	}
	return data, nil
}

// wrap a http handler with some error handling
// so if it returns an error we handle it
func Wrapper[T any](handler httpWrapper[T]) func(res http.ResponseWriter, req *http.Request) {
	ret := func(res http.ResponseWriter, req *http.Request) {
		data, err := handler(res, req)
		if err != nil {
			log.Ctx(req.Context()).Error().Msgf("error for route: %s", err.Error())
			httpError, ok := err.(HTTPError)
			if ok {
				http.Error(res, httpError.Error(), httpError.StatusCode)
			} else {
				http.Error(res, err.Error(), http.StatusInternalServerError)
			}
			return
		} else {
			err = json.NewEncoder(res).Encode(data)
			if err != nil {
				log.Ctx(req.Context()).Error().Msgf("error for json encoding: %s", err.Error())
				http.Error(res, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}
	return ret
}
