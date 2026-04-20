GROUT_VERSION=4.8.1.0
GROUT_URL=https://github.com/rommapp/grout/releases/download/v$GROUT_VERSION/Grout-RetroDECK.zip

NONSTEAM_VERSION=0.7.0
NONSTEAM_URL=https://github.com/cameronhimself/nonsteam/releases/download/$NONSTEAM_VERSION/nonsteam-linux-x64-$NONSTEAM_VERSION.tar.gz

echo "Creating installation directory..."
mkdir -p "$HOME/grout" && cd "$HOME/grout"
export PATH="$HOME/grout:$PATH"

echo "Downloading and extracting Grout..."
curl -sL -o grout.zip $GROUT_URL
unzip  -d . -o -qq grout.zip
chmod +x Grout.sh Grout/grout
rm grout.zip

echo "Downloading and extracting nonsteam..."
curl -sL -o nonsteam.tgz $NONSTEAM_URL
tar -xzf nonsteam.tgz --strip-components=1
rm nonsteam.tgz

echo "Adding Grout as a non-Steam game..."
nonsteam add -w \
  --app-name "Grout" \
  --exe "env" \
  --start-dir "$HOME/grout/" \
  --launch-options "$HOME/grout/Grout.sh" \
  --image-icon "$HOME/grout/Grout/media/icon.png" \
  --image-grid "$HOME/grout/Grout/media/cover.png" \
  --image-grid-horiz "$HOME/grout/Grout/media/banner.png" \
  --image-hero "$HOME/grout/Grout/media/background.png" \
  --image-logo "$HOME/grout/Grout/media/logo.png" \
  --allow-overlay

echo "Cleaning up..."
rm nonsteam

echo "Done! Please restart Steam for changes to take effect."
