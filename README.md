LogDNA Logrus and Logrus Mate library
=====================================

[![Go Report Card][grc-badge]][grc]

This library allows logging to [LogDNA][logdna] from [Logrus][logrus],
configuring everything using [Logrus Mate][mate] (currently, the only way).

This works for me, but is currently a work in progress.

Configuration
-------------

Check out the `example/` subdirectory for a simple example.

Currently, the only way to set up hook is to use Logrus Mate.
For HOCON configuration provider (the default), add a hook like this:

    your-logger {
        ...
        hooks {
            logdna {
                api-key = "your-ingestion-key-here"
                app = "myapp"
                json = true
            }
            ...
        }
        ...
    }

The supported options are:

  - `api-key`: ingestion key, mandatory
  - `hostname`: hostname override, if not specified will use `os.Hostname()`
  - `mac`: host MAC address, not sent if empty, no autodetection for now
  - `ip`: host IP address, ditto
  - `app`: app identifier, for LogDNA to filter
  - `env`: environment identifier, for LogDNA to filter
  - `size`: buffer size, in entries, defaults to 4096
  - `flush`: flush interval, defaults to 10 seconds
  - `qsize`: buffered channel size
  - `drop`: whenever hook's allowed to drop messages (if buffer's full etc).
    Logging will block if this is false and there are size+qsize entries
    waiting to be sent.
  - `json`: how to send structured log fields. Will format line as JSON if set
    to true (using `"message"` key for log message), or pass fields as `meta`
    if not.
  - `url` allows to override the ignestion endpoint URL (e.g. for testing)

Limitations
-----------

As the logging is asynchronous, error handling is limited.
If `drop` is set to true, you can lose messages on outages.
If `drop` is false, program may panic at shutdown, or buffer size
may grow past the `size` limit.

Also, to give the hooks chance flush before the program exits,
don't forget to add `defer logrus.Exit(0)` to your `main`.

Suggestions (and pull requests) are welcomed.

[logdna]: https://logdna.com
[logrus]: https://github.com/sirupsen/logrus
[mate]: https://github.com/gogap/logrus_mate
[grc-badge]: https://goreportcard.com/badge/github.com/drdaeman/logdna-logrus
[grc]: https://goreportcard.com/report/github.com/drdaeman/logdna-logrus
