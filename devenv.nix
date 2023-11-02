{ pkgs, ... }:

{
  # https://devenv.sh/packages/
  packages = [
    pkgs.go
    pkgs.git
    pkgs.govulncheck
    pkgs.gopls
    pkgs.golint
  ];
}
