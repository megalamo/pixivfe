#!/usr/bin/fish

# see the files touched by ./build.sh

for cmd in build build_binary check_css i18n_extract i18n_merge i18n_validate
	set f (mktemp)
	trap "rm $f" EXIT
	./build.sh $cmd
	find . -type f -newer $f > /tmp/pixivfe-mtime-$cmd
end
