#!/bin/sh
# Applies patches from the patches/ directory to newly uploaded toolchains.
# Assumes patches are ordered numerically in the order they should be applied.

PATCHES=patches/*

if [ $# -eq 0 ]; then
       echo Usage: $0 [path_to_patch]
       echo Example: $0 1.46.0
       exit 1
fi

echo Applying patches to $1
for patch in $PATCHES
do
	echo ----- Applying $patch
	patch -p3 -d $1 < $patch
done
