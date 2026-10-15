# OrgDoc security model

## Trust boundary

OrgDoc runs capability workers as child processes under the current user's operating-system identity. Install and run only worker code trusted by the organization. The current runner is a process orchestrator, not a sandbox.

## Controls enforced by the runtime

- Capability manifests, organization profiles, document inputs, and worker protocol results are validated before they are used at their respective boundaries.
- Workers are invoked directly with an argument vector. OrgDoc does not interpolate an entrypoint through a shell.
- Every job receives a newly created artifact directory with owner-only permissions. Existing directories are refused.
- The runner applies the capability timeout, supports caller cancellation, and limits concurrent jobs by declared resource class. The default limit for each class is one.
- Combined worker stdout and stderr are bounded by `max_output_bytes`. Output paths reported in the protocol must be relative, resolve inside the job artifact directory, and identify regular files.
- The child environment is filtered through an explicit allowlist. It preserves `PATH`, user and temporary-directory settings, locale, selected XDG and model-cache paths, `CUDA_HOME`/`CUDA_VISIBLE_DEVICES`, Windows process-required path variables, and selected `MEDIA_REPORT_*` settings, including `MEDIA_REPORT_OPENAI_API_KEY`. Other `MEDIA_REPORT_*` names, `HF_TOKEN`, `PYTHONPATH`, `PYTHONHOME`, `AWS_*`, and proxy variables are excluded unless explicitly added to the allowlist.
- Common credential forms in worker stderr and structured diagnostic messages are redacted, including authorization headers, bearer/basic values, API keys, tokens, passwords, and client secrets in common `key=value` and JSON forms.
- Remote LLM use is explicit in the selected media configuration and emits a warning. Credentials are passed to the worker only through the documented provider setting or the user's existing configuration file.

Redaction is a precaution for common formats, not complete secret detection. Do not put credentials in worker input, filenames, output paths, or arbitrary logs.

## Guarantees the runner does not provide

- A worker inherits the user's filesystem permissions and can read or write any path that user can access. `HOME`, configuration directories, and `PATH` are intentionally available; a worker can read user configuration files and credentials.
- A worker can access the network, start other processes, consume disk, and create side effects outside its artifact directory. The runner's output-path validation applies only to files the worker reports.
- Manifest `permissions` describe intended access. They do not restrict filesystem or network operations.
- Resource classes limit concurrent OrgDoc jobs only. They do not impose CPU, memory, disk, GPU, process-count, or network quotas on a worker.
- Output bounds limit captured stdout/stderr. They do not cap files written to disk or data sent over the network.
- Checksums detect archive corruption when compared with the release manifest. They do not authenticate the publisher or prove that a worker is safe. Release signing and publisher identity are not currently implemented.
- Environment filtering reduces accidental inheritance but does not isolate the worker. Allowlisted values such as `PATH`, `HOME`, and provider credentials remain powerful.

## Timeout and cancellation behavior

On Linux and macOS, OrgDoc starts each worker in its own process group and kills that group when a timeout or cancellation occurs. On other platforms, including experimental Windows support, OrgDoc uses Go's standard command cancellation, which terminates the direct child but does not guarantee that descendants also terminate. Prefer Linux or macOS for workflows that depend on process-tree cleanup.

## Future isolation

Any third-party worker distribution requires a separate design for publisher identity, signatures, immutable versions, compatibility, dependency provenance, uninstall and rollback, permission review, network policy, and OS-enforced isolation. Possible OS controls include Linux namespaces/seccomp or a container runtime and macOS sandbox profiles. They are future work; OrgDoc does not claim those boundaries today.
