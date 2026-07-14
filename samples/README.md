# Safe demonstration corpus

These samples contain no malware, exploits, backdoors, macros, or executable code. Suspicious samples contain only synthetic marker strings in ignored comments or metadata so DataGuardian can demonstrate deterministic findings safely.

| Sample | Expected result |
|---|---|
| `clean/*` | Accepted, LOW risk, no suspicious marker findings |
| `suspicious-inert/pdf-js-markers.pdf` | HIGH; PDF JavaScript marker, OpenAction marker, and generic synthetic markers |
| `suspicious-inert/png-encoded-marker.png` | MEDIUM; generic base64-like and `eval(` markers |
| `suspicious-inert/jpeg-encoded-marker.jpg` | MEDIUM; generic base64-like and `eval(` markers |
| `suspicious-inert/jpeg-exif-gps.jpg` | HIGH; fictional EXIF GPS plus synthetic camera/date metadata |
| `suspicious-inert/text-eval-marker.txt` | MEDIUM; generic base64-like and `eval(` markers |
| `rejected/mismatched-extension.jpg` | Rejected because detected PNG content does not match `.jpg` |
| `rejected/malformed.pdf` | Rejected because detected text content does not match `.pdf` |
| `rejected/empty.txt` | Rejected because empty uploads are not accepted |

All identities and content are fictional. Verify files with `sha256sum -c samples/CHECKSUMS.sha256` from the `samples` directory or regenerate them with:

```sh
python3 scripts/generate_safe_samples.py
python3 scripts/generate_safe_samples.py --check
```

Never replace these fixtures with real malware or weaponized documents. Do not open untrusted user submissions merely because these demonstration files are harmless.
