{
  description = "fwatcher dev workspace";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        devShells.default = pkgs.mkShell {
          # hardeningDisable = [ "all" ];

          buildInputs = with pkgs; [
            # source version control
            git
            pre-commit

            go_1_22
            gotestfmt

            upx
            go-task
          ];

          shellHook = ''
          '';
        };
      }
    );
}


