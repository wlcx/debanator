{
  description = "Serve your deb files as an APT repo";

  inputs.utils.url = "github:numtide/flake-utils";
  inputs.devshell.url = "github:numtide/devshell";
  inputs.nixpkgs.url = "github:nixos/nixpkgs";

  outputs = {
    self,
    nixpkgs,
    utils,
    devshell,
  }:
    utils.lib.eachDefaultSystem (system: let
      pkgs = import nixpkgs {
        inherit system;
        overlays = [devshell.overlays.default];
      };
    in rec {
      packages.default = pkgs.buildGoModule {
        name = "debanator";
        src = self;
        vendorSha256 = "sha256-M8/8fi0JBkUDjhLpur54bv2HzaOEhAIuqZO+oSvkIBk=";

        ldflags = ''
          -X debanator.Commit=${if self ? rev then self.rev else "dirty"}
        '';
      };

      apps.default = utils.lib.mkApp {drv = packages.default;};

      devShells.default =
        pkgs.devshell.mkShell {packages = with pkgs; [go gopls goreleaser];};
      formatter = pkgs.alejandra;
    });
}
