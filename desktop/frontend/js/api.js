// Go methods bound in desktop.Run are exposed by the Wails runtime as
// window.go.<package>.<Struct>.<Method>() and return Promises. A Go error
// rejects the Promise with its message.
export const api = window.go.desktop.API;
