# gitproject

`gitproject` is a narrow control-plane adapter between Yunka project-relative paths and Git repository-relative paths.

It derives the repository root and nested project prefix from the current checkout using Git. It does not persist another project path model or source of truth.

Callers keep project-relative paths in Yunka evidence and translate only at Git tree/diff boundaries.
