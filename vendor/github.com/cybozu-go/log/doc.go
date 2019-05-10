/*
Package log provides the standard logging framework for cybozu products.

As this is a framework rather than a library, most features are hard-coded
and non-customizable.

cybozu/log is a structured logger, that is, every log entry consists of
mandatory and optional fields.  Mandatory fields are:

    "topic" is by default the executables file name w/o directory path.
    "logged_at" is generated automatically by the framework.
    "severity" corresponds to each logging method such as "log.Error".
    "utsname" is generated automatically by the framework.
    "message" is provided by the argument for logging methods.

To help development, logs go to standard error by default.  This can be
changed to any io.Writer.  Logs are formatted by a Formatter.  Following
built-in formatters are available.

    Plain (default): syslog like text formatter.
    logfmt:          https://gist.github.com/kr/0e8d5ee4b954ce604bb2
    JSON Lines:      http://jsonlines.org/

The standard field names are defined as constants in this package.
For example, "secret" is defined as FnSecret.

Field data can be any type though following types are recommended:

    nil,
    bool,
    time.Time (formatted in RFC3339),
    string and slice of strings,
    int, int8, int16, int32, int64, and slice of them,
    uint, uint8, uint16, uint32, uint64, and slice of them,
    float32, float64, and slice of them,
    map[string]interface{} where values are one of the above types.

The framework automatically redirects Go's standard log output to
the default logger provided by this framework.
*/
package log
