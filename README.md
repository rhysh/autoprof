# Autoprof

Autoprof provides tools for Go programs to assemble runtime profiles of themselves.

You might like it if:
- Go is a core language for your team
- You want your programs to be more efficient or reliable
- You've collected profiles from a variety of sources and then forgotten which is which
- Someone has shared a profile with you, but you don't know which machine or commit produced it
- You'd really love to see a profile from during that recent outage
- Even if some of your apps aren't HTTP servers with their own listening port
- Or they run in permission-constrained or non-Linux environments

It describes a file and directory structure for organizing these self-collected profiles.
That helps with keeping track of where the profiles came from, even if you've got hundreds of these snapshots from one instance of an app at different timestamps, or if you've gathered snapshots from thousands of instances of your app.
Or both!

The "Auto" part of the name is because the applications profile themselves.
That removes the administrative overhead of running an agent and a collection service, with no extra permissions (from the OS, or perhaps anyone else).

The contents of each profile bundle are quite close to what you'd get from using the `net/http/pprof` package, with minimal changes along the way.
You don't need to understand much about what the Autoprof library is doing to the profiles before you read them: there are no transformations.

## What does the data look like?

Autoprof builds profiles into "archives" or "bundles", which are uncompressed zip files.

They start with a JSON blob called "./meta", which describes the program instance that created the data.

Next is a JSON blob called "./expvar", which includes the key-value pairs the app has registered with the `expvar` package, as you could get with an HTTP `GET /debug/vars` request to the `http.DefaultServeMux` (as a side-effect of importing the `expvar` package).

Then, a heap profile, protobuf-encoded and gzipped, as you could get from an HTTP `GET /debug/pprof/heap?debug=0` request to the `http.DefaultServeMux` (as a side-effect of importing the `net/http/pprof` package).
It's named "./pprof/heap".

Then, all the other point-in-time profiles known to the `runtime/pprof` package.
That includes the goroutine profile and the mutex and block profiles (did you activate them by calling `runtime.SetMutexProfileFraction` and `runtime.SetBlockProfileRate`?).
It includes the "allocs" profile even though it includes all the same info as the "heap" profile, and the "threadcreate" profile even though it's been broken for a while.
(Maybe those two should change.)
It includes all the other custom point-in-time snapshot profiles that may have been registered with the runtime/pprof package.
All of these are in the "./pprof/" directory. Their names are url path encoded, since custom profile names may include "/".

After all of those point-in-time snapshots, a bundle may include a CPU profile, an execution trace, or both.
These are named "./pprof/profile" and "./pprof/trace", again following the naming you'd expect from `net/http/pprof`.
When both are enabled, the execution trace will include timestamped CPU profile samples, and the bundle will include that additional CPU profile as "./pprof/profile-during-trace".
(Maybe that name should change.)

## How does it compare with `net/http/pprof`?

First, it's easy to lose track of where the profile came from if it's been more than a few minutes since you downloaded it.
Autoprof's data bundles include a reference to the original source of the data: the application name and version, the host or container name, a unique ID for the process instance, and a convenient timestamp.

Second, you may not have interactive access to the app: it may run in a private network or on a customer device, or the app might be something other than a long-running HTTP server.

Third, to provide historical data.
You can set up Autoprof to create a bundle on a regular schedule, saving it to the local filesystem or your favorite blob store, ready for review if and when you need it.
When something breaks, you can focus on restoring service instead of frantically downloading profiles for later debugging.

Collecting on a schedule also means addressing risks up front: if profiling leads to instability or excessive overhead in your app, you'll discover that early on while you're not simultaneously trying to solve some other problem.

## How does it compare with other continuous profilers?

Autoprof lets you dip your toe into continuous profiling without up-front administrative work.
There's no agent to deploy, and no centralized collection service to arrange.
You can use it on your development machine, writing data to the local filesystem.
Or in production, to the local filesystem or a blob store.
(Maybe you have one of those available already?)

It collects profiling data in Go's native formats.
This reduces the chance that any strange-looking data is an artifact of the collection code: you can focus on understanding your app rather than second-guessing the data.
It also makes the full power of Go's profiles (goroutine labels, execution traces, etc) available to you.

And because Autoprof doesn't need to cross process boundaries or ask the kernel to do any especially privileged operations, it can run just about anywhere.
That includes non-Linux and permission-constrained Linux environments, where the amazing new powers of eBPF aren't an option.

## Caveats

Autoprof's CPU profiles give a fair overview of the app's regular work, but they won't show the cost of Autoprof's own work to collect the profiles: for example, it always collects the heap and goroutine profiles before it starts the CPU profile, and it always finalizes and stores the data bundle after stopping the CPU profile.
For the first part, the Go runtime includes a timestamp inside each protobuf-encoded profile; the deltas between those timestamps can give a view into how long the app takes to assemble the bundle (including waiting for on-CPU time).

The entire bundle is buffered within the app's own memory until it's finalized and stored.
For small heaps (or with large execution traces), it may affect the garbage collector's pacing.

## Does anyone use this in production?

Yes, since 2018 (circa Go 1.10).
The largest change since then (before its public release) was probably to add support for the simultaneous combination of CPU profiles and execution traces (Go 1.19).
It's also had some recent work to untangle it from a couple of non-public libraries (for determining hostname, app name, app version, etc), and to not depend on any particular blob storage SDK.

## How can I contribute?

Most of all, please share stories of how you use this package, and what sorts of problems you use it to solve -- or what you wish it could solve.

Get to the bottom of problems in your apps, and problems in the packages you use.
