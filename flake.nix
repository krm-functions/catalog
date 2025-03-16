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
        packages.default = self.packages.${system}.krm-package-compositor;
        packages.krm-package-compositor = pkgs.buildGoModule {
          pname = "krm-package-compositor";
          version = "0.2.0";
          src = ./.;
          subPackages = [ "cmd/package-compositor" ];
          vendorHash = "sha256-IPEMf1fLvFQTsZwprswo+HUX3HuFLhO6og2yPPKkVYg=";
          # vendorHash = nixpkgs.lib.fakeHash;
          buildInputs = [
            # ...
          ];
          postInstall = ''
            mv $out/bin/package-compositor $out/bin/krm-package-compositor
          '';
        };
      }
    );
}
