<!-- trunk-ignore-all(markdownlint/MD033) -->
<!-- trunk-ignore(markdownlint/MD041) -->
<div align="center">

  <img src=".github/resources/isotipo.png" height="180px" width="auto" alt="romm-grout logo">
  <br />
  <img src=".github/resources/logotipo.png" height="45px" width="auto" alt="romm grout logotype">
    <h3 style="font-size: 25px;">
      A Download client for NextUI and muOS
  </h3>

<br>

[![license-badge-img]][license-badge]
[![release-badge-img]][release-badge]
[![discord-badge-img]][discord-badge]

  </div>
</div>

## How do I setup Grout?

### NextUI Setup

1. Own a TrimUI Brick or Smart Pro and have a SD Card with NextUI.
2. Connect your device to a Wi-Fi network.
3. The preferred Grout installation method is to use the NextUI Pak Store. You can find the Pak Store in the
   `Tools` menu. Once in the Pak Store, Grout can be found under the `ROM Management` category.
    - Alternatively, download the [latest Grout release](https://github.com/rommapp/grout/releases/latest) for
      NextUI (look for
      `Grout.pak.zip`)
    - For manual downloads, extract the release zip and place the `Grout.pak` directory into `SD_ROOT/Tools/tg5040`.
4. Launch `Grout` from the `Tools` menu and enjoy!

**Note:** NextUI is only currently supported on the TrimUI Smart Pro and TrimUI Brick. These systems will have controls
automatically mapped.

### muOS Setup

Grout has only been tested on muOS 2508.1 Canada Goose on an Anbernic RG35XXSP.

Please help by verifying if it works on other devices!

1. Own a supported device running muOS.
2. Download the [latest Grout release](https://github.com/rommapp/grout/releases/latest) for muOS (look for
   `Grout.muxapp`).
3. Transfer the `Grout.muxapp` file to SD1 `(mmc)/ARCHIVE` on your device.
4. Go to Applications and launch Archive Manager.
5. Select [SD1-APP] Grout from the list and let it extract to your applications directory.
6. Exit Archive Manager.
7. Find an [input mapping config](/.github/resources/input_mappings) for your device.
    - If one does not exist, please try one for a different device.
    - If that does not work, please [create an issue](https://github.com/rommapp/grout/issues/new).
    - A first launch setup process is in the works but is not ready for prime-time.
8. Save the input mapping JSON file as `input_mapping.json` and transfer it to SD1 `(mmc)/Applications/Grout` on your
   device.
9. Select `Apps` on the Main Menu, launch Grout and enjoy!

**Note:** Grout does not support downloading art on muOS. This will be added in a future release.

---

### Development Notes

To run Grout locally, you will need to have Go installed on your system.

TODO: Flesh this out.

## Be a friend, tell a friend something nice; it might change their life!

I've spent a good chunk of time building Grout.

If you feel inclined to pay it forward, go do something nice for someone! ❤️

✌🏻

<!-- Badges -->

[license-badge-img]: https://img.shields.io/github/license/rommapp/grout?style=for-the-badge&color=a32d2a

[license-badge]: LICENSE

[release-badge-img]: https://img.shields.io/github/v/release/rommapp/grout?style=for-the-badge

[release-badge]: https://github.com/rommapp/grout/releases

[discord-badge-img]: https://img.shields.io/badge/discord-7289da?style=for-the-badge

[discord-badge]: https://discord.gg/P5HtHnhUDH

<!-- Links -->

[discord-invite]: https://invidget.switchblade.xyz/P5HtHnhUDH

[discord-invite-url]: https://discord.gg/P5HtHnhUDH

[oc-donate-img]: https://opencollective.com/romm/donate/button.png?color=blue

[oc-donate]: https://opencollective.com/romm
