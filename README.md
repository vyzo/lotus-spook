# lotus-spook
A simple tool for snarfing peer records from the bootstrappers throug PX.

Run:
```
$ go install ./...
$ spook -q
```

To run with more embedded peers, use `-n`; don't use more than 5 peers, you'll trip the IP colocation factor and get no PX.

To run with a persistent identity, use `-id` and pass a base path.

To connect to a specific bootstrapper, use `-b` with a coma separated list of bootstrappers.

To log the discovered peers, use `-f` with an output file.
