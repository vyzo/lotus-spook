# lotus-spook
A forensics tool for snarfing peer ID/IP tuples from the bootstrappers

Run:
```
$ go install ./...
$ lotus-spook -q
```

To run with more embedded peers, use `-n`; don't use more than 5 peers, you'll trip the IP colocation factor and get no PX.
