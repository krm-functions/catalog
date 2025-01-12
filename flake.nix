{
  description = "Flake for krm-functions/catalog";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let pkgs = nixpkgs.legacyPackages.${system}; in
      {
        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            gnumake
            go_1_23
            golangci-lint
          ];
        };
        # packages.default = self.packages.krm-package-compositor;
        packages.default = pkgs.buildGoModule {
          pname = "package-compositor";
          version = "0.1.0";
          src = ./.;
          subPackages = [ "cmd/package-compositor" ];
          vendorHash = "sha256-wWOchYWReHnYqGaEk43RDNcxOTL6RWSL1US+6uA/kbg=";
          #vendorHash = nixpkgs.lib.fakeHash;
          buildInputs = [
            # ...
          ];
        };
      }
    );
}
