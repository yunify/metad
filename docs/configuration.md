# Configuration and Command line flags  Guide

The metad configuration file is written in YAML, and is optional. The command line flags can override the configuration file.

Configuration option and command line flags table

| Configuration Option          |Command line flag | Default        | Description  |
| ------------------------------|:-----------------| :--------------|--------------|
|                               | --version        | false          |Show metad version|
|                               | --config         |                |The configuration file path|
| backend                       | --backend        | local          |The metad backend type|
| nodes                         | --nodes          |                |List of backend nodes|
| log_level                     | --log_level      | info           |Log level for metad print out: debug\|info\|warning |
| pid_file                      | --pid_file       |                |PID to write to|
| xff                           | --xff            | false          |X-Forwarded-For header support|
| prefix                        | --prefix         |                |Backend key path prefix|
| group                         | --group          | default        |The metad's group name, same group share same mapping config from backend|
| only_self                     | --only_self      | false          |Only support self metadata query|
| listen                        | --listen         | :80            |Address to listen to (TCP)  |
| listen_manage                 | --listen_manage  | 127.0.0.1:9611 |Address to listen to for manage requests (TCP) |
| basic_auth                    | --basic_auth     | false          |Use Basic Auth to authenticate (only used with --backend=etcd\|etcdv3)|
| client_ca_keys                | --client_ca_keys |                |The client ca keys (for etcd\|etcdv3) |
| client_cert                   | --client_cert    |                |The client cert (for etcd\|etcdv3)|
| client_key                    | --client_key     |                |The client key (for etcd\|etcdv3)|
| username                      | --username       |                |The username to authenticate as (for etcd\|etcdv3) |
| password                      | --password       |                |The password to authenticate with (for etcd\|etcdv3) |

>Note: Command line bool flag can not to use '--xff=true' format, flag appear means true, otherwise false. 
