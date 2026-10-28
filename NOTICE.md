# Polaris: copyright and additional terms

Polaris
Copyright (C) 2026 AstraLumi <https://github.com/AstraLumi>

This program is free software: you can redistribute it and/or modify it
under the terms of the GNU Affero General Public License as published by the
Free Software Foundation, version 3 of the License (AGPL-3.0-only), together
with the additional terms below. It is distributed in the hope that it will
be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. The full license is in
[LICENSE](LICENSE).

## Additional terms (AGPL-3.0, section 7)

These terms apply to Polaris and to every copy and modified version of it.

1. **Attribution** (section 7(b)). Every copy or modified version that you
   convey, or that you let others use over a network, must keep a visible
   author attribution reading "Based on Polaris by AstraLumi", linking to
   <https://github.com/AstraLumi/polaris>:
   1. in its README, or its main documentation if it has no README; and
   2. in its user interface, among its Appropriate Legal Notices: where
      Polaris shows it (Settings → About Polaris), or, if that page is
      removed, in an equivalent place that users can reach.

2. **Modified versions** (section 7(c)). A modified version must be marked
   as changed from the original, and must not be presented as Polaris
   itself or as made, provided or endorsed by AstraLumi.

3. **Name and logo** (section 7(e)). This license grants no rights under
   trademark law to the name "Polaris" or the Polaris logo. A modified
   version that you convey, or let others use over a network, must use its
   own name and logo; it may say that it is "based on Polaris". An
   unmodified copy may keep the name, but must not suggest that it is
   provided or endorsed by AstraLumi.

## Third-party data

The writing editor's spellchecker ships Hunspell dictionaries from the
LibreOffice dictionaries repository
(<https://github.com/LibreOffice/dictionaries>, revision
`4f1df64b2e8e14751aa7c2205d170ca016071315`), unmodified but gzipped, in
`frontend/public/dict/`. They are separate works distributed alongside
Polaris under their own licences; each folder keeps the dictionary's own
readme and licence files.

- `en_US`: the SCOWL-derived en_US dictionary by Kevin Atkinson and
  others (README_en_US.txt): permissive terms (SCOWL/VarCon notices, an
  Ispell BSD-style licence), which must be kept with copies.
- `en_GB`: the British English dictionary by David Bartlett, Brian Kelk,
  Andrew Brown and Marco A.G. Pinto (README_en_GB.txt), under the GNU
  LGPL (version 3, per its changelog).
- `pt_BR`: VERO, the Brazilian Portuguese dictionary by Raimundo Moura and
  the LibreOffice Brazilian community (README_pt_BR.txt, README_en.txt),
  under the GNU LGPL version 3 and the Mozilla Public License.
- `es_ES`: the RLA-ES Spanish dictionary coordinated by Santiago Bosio
  (README_hunspell_es.txt, LICENSE.md), under your choice of the GNU GPL
  version 3 or later, the GNU LGPL version 3 or later or the MPL 1.1 or
  later.

The licence texts of the GNU GPL 3, the GNU LGPL 3 (which en_GB and
pt_BR use too) and the MPL 1.1 are in `frontend/public/dict/es_ES/`
(GPLv3.txt, LGPLv3.txt, MPL-1.1.txt). The dictionaries' source is the
repository above; `frontend/scripts/dictionaries.mjs` fetches them again.
