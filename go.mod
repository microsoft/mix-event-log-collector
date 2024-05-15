module nuance.xaas-logging.event-log-collector

go 1.22.3

require (
	github.com/IBM/sarama v1.41.2
	github.com/alexflint/go-arg v1.4.3
	github.com/anu1097/golang-masking-tool v0.0.6
	github.com/apache/pulsar-client-go v0.11.0
	github.com/common-nighthawk/go-figure v0.0.0-20210622060536-734e95fb86be
	github.com/elastic/go-elasticsearch/v7 v7.17.10
	github.com/go-audio/audio v1.0.0
	github.com/go-audio/wav v1.1.0
	github.com/go-redis/redis/v8 v8.11.5
	github.com/google/uuid v1.1.2
	github.com/jellydator/ttlcache/v3 v3.1.0
	github.com/lestrrat-go/fluent-client v1.0.0
	github.com/natefinch/lumberjack v2.0.0+incompatible
	github.com/opensearch-project/opensearch-go v1.1.0
	github.com/prometheus/client_golang v1.11.1
	github.com/redis/go-redis/v9 v9.0.5
	github.com/reugn/go-streams v0.9.1-0.20230930135953-1049716797ed
	github.com/reugn/go-streams/kafka v0.1.3-0.20230930135953-1049716797ed
	github.com/reugn/go-streams/pulsar v0.1.3-0.20230930135953-1049716797ed
	github.com/reugn/go-streams/redis v0.1.3-0.20230930135953-1049716797ed
	go.mongodb.org/mongo-driver v1.12.1
	go.uber.org/zap v1.26.0
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1
	heckel.io/pqueue v1.0.0
)

require (
	github.com/99designs/go-keychain v0.0.0-20191008050251-8e49817e8af4 // indirect
	github.com/99designs/keyring v1.2.1 // indirect
	github.com/AthenZ/athenz v1.10.39 // indirect
	github.com/DataDog/zstd v1.5.0 // indirect
	github.com/alexflint/go-scalar v1.1.0 // indirect
	github.com/ardielle/ardielle-go v1.5.2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bits-and-blooms/bitset v1.4.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/danieljoos/wincred v1.1.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/dvsekhvalnov/jose2go v1.7.0 // indirect
	github.com/eapache/go-resiliency v1.4.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20230731223053-c322873962e3 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/go-audio/riff v1.0.0 // indirect
	github.com/godbus/dbus v0.0.0-20190726142602-4481cbc300e2 // indirect
	github.com/golang-jwt/jwt v3.2.1+incompatible // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/gsterjov/go-libsecret v0.0.0-20161001094733-a6f4afe4910c // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.7.6 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.4 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/klauspost/compress v1.16.7 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.3 // indirect
	github.com/lestrrat-go/backoff/v2 v2.0.3 // indirect
	github.com/lestrrat-go/bufferpool v0.0.0-20180220091733-e7784e1b3e37 // indirect
	github.com/lestrrat-go/msgpack v1.0.0 // indirect
	github.com/lestrrat-go/option v0.0.0-20210103042652-6f1ecfceda35 // indirect
	github.com/lestrrat-go/pdebug v0.0.0-20200204225717-4d6bd78da58d // indirect
	github.com/linkedin/goavro/v2 v2.9.8 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/montanaflynn/stats v0.0.0-20171201202039-1bf9dbcd8cbe // indirect
	github.com/mtibben/percent v0.2.1 // indirect
	github.com/pierrec/lz4 v2.0.5+incompatible // indirect
	github.com/pierrec/lz4/v4 v4.1.18 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.26.0 // indirect
	github.com/prometheus/procfs v0.6.0 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475 // indirect
	github.com/rogpeppe/go-internal v1.11.0 // indirect
	github.com/sirupsen/logrus v1.6.0 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.2 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20181117223130-1be2e3e5546d // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/crypto v0.23.0 // indirect
	golang.org/x/mod v0.9.0 // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/oauth2 v0.0.0-20210402161424-2e8d93401602 // indirect
	golang.org/x/sync v0.3.0 // indirect
	golang.org/x/sys v0.20.0 // indirect
	golang.org/x/term v0.20.0 // indirect
	golang.org/x/text v0.15.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.34.1 // indirect
)
