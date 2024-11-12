# pkg/status

<!-- status autogenerated section -->
| Status        |           |
| ------------- |-----------|
| Stability     | [development]: extension   |
| Issues        | [![Open issues](https://img.shields.io/github/issues-search/open-telemetry/opentelemetry-collector-contrib?query=is%3Aissue%20is%3Aopen%20label%3Apkg%2Fstatus%20&label=open&color=orange&logo=opentelemetry)](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues?q=is%3Aopen+is%3Aissue+label%3Apkg%2Fstatus) [![Closed issues](https://img.shields.io/github/issues-search/open-telemetry/opentelemetry-collector-contrib?query=is%3Aissue%20is%3Aclosed%20label%3Apkg%2Fstatus%20&label=closed&color=blue&logo=opentelemetry)](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues?q=is%3Aclosed+is%3Aissue+label%3Apkg%2Fstatus) |
| [Code Owners](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/CONTRIBUTING.md#becoming-a-code-owner)    | [@jpkrohling](https://www.github.com/jpkrohling), [@mwear](https://www.github.com/mwear) |

[development]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#development
<!-- end autogenerated section -->

## Overview

This module contains functionality for aggregating status information about components. Used by extensions that
implement the `componentstatus.Watcher` interface for collecting status information about the internal status of all
components inside the collector.

### History

`pkg/status` was originally developed for the `healthcheckv2extension`. It provided a generic way of aggregating
status information for components. Other extensions that need to implement the `componentstatus.Watcher` can now use
the same implementation `healthcheckv2extension` uses.