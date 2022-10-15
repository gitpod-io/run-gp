#!/bin/sh
set -u; # To error on undefined variables
# A POSIX compliant shell script to easily install run-gp from your terminal

log() {
	kind="${1:-"log"}" && shift
	printf "[%%%] \033[1;37m${kind}\033[0m: %s\n" "$@";
}

checkshellrc_key() {
	grep -q -- "source.*\.gitpod/env" "$1" 2>/dev/null;
}

selfinstall() {

	# Ensure HOME is set
	if test -z "${HOME:-}"; then {
		log error "\$HOME environment variable is not set";
		exit 1;
	} fi
	self_home="$HOME/.gitpod";
	app="run-gp"

	# Locate a writable PATH
	while IFS=':' read -r path; do {
		# Check if PATH exists and is writable
		if test -w "$path"; then {
			target_install_dir="$path";
			break;
		} fi
	} done <<-TOREAD
		$PATH
	TOREAD

	# Check if we were not able to fetch an usable installation path
	if test -z "${target_install_dir:-}"; then {
		target_install_dir="$self_home/bin" && self_created_path=true;
		log warn "Failed to retrieve a writable directory from \$PATH";
		log info "Falling back to $target_install_dir";
	} fi

	## At this stage we are good to go

	# Inject gitpod env file for future usage
	posix_envfile="$self_home/env.sh";
	fish_envfile="$self_home/env.fish";

	for shellrc in ".bashrc" ".config/fish/config.fish" ".kshrc" ".zshrc"; do {

		if ! checkshellrc_key "$shellrc"; then {	
			mkdir -p "${shellrc%/*}" || {
				log error "Failed to create dir for $shellrc";
				exit 1;
			};

			case "$shellrc" in
				*"shrc") # bash, ksh, zsh
					printf 'source "%s";\n' "$posix_envfile" >> "$shellrc";
				;;
				*"fish") # fish
					printf 'source "%s";\n' "$fish_envfile" >> "$shellrc";
				;;
			esac

			if test $? != 0; then {
				log warn "Failed to update $shellrc"
			} fi

		} fi

	} done

	log info "Installing $app to $target_install_dir";
	target_full_path="$target_install_dir/$app";
	rm -f "$target_full_path" 2>/dev/null || :; # Necessary, in case its originating from a dead symlink

	# TODO: Detect OS and CPU-arch
	# TODO: Check whether curl or wget is available
	# TODO: Finally download the binary

	chmod +x "$target_full_path" || {
		log error "Failed to mark $target_full_path as executable";
		exit 1;
	};

	if test "${self_created_path:-}" == true; then {
		log info "Restart your shell to update \$PATH environment for $app" \
					"Later you can run '$app --help' to get started";
	} else {
		log info "Installation complete, run '$app --help' to get started";
	} fi
}
