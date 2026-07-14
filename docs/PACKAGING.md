# Packaging Guidance

DataGuardian remains Docker Compose-first. Native packages should wrap the documented Compose workflow rather than bundle a second runtime.

Before publishing `.deb`, Windows, or macOS installers:

- pin container images by digest for a release;
- generate an SBOM and publish checksums;
- sign release archives and installer metadata;
- preserve volumes during upgrades and document backup/restore;
- never embed production credentials, JWT keys, or `.env` files;
- test upgrades from the previous supported release;
- keep the backend bound only to intended interfaces;
- retain the passive, non-executing inspection model.

Unsigned native installers should not be presented as production-ready downloads.
