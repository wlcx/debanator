{
  description = "Another cool golang abhorration from samw";

  inputs.utils.url = "github:numtide/flake-utils";
  inputs.devshell.url = "github:numtide/devshell";

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
        name = "my-project";
        src = self;
        vendorSha256 = "";

        # Inject the git version if you want
        #ldflags = ''
        #  -X main.version=${if self ? rev then self.rev else "dirty"}
        #'';
      };

      apps.default = utils.lib.mkApp {drv = packages.default;};

      devShells.default =
        pkgs.devshell.mkShell {packages = with pkgs; [go gopls];};
      formatter = pkgs.alejandra;
    });
}
