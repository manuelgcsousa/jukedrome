{
  pkgs ? import <nixpkgs> { },
}:

pkgs.mkShell {
  packages = [
    pkgs.go
    pkgs.just
    pkgs.fzf
    pkgs.mpv
  ];
}
