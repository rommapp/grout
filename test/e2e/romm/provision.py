"""Bring a fresh RomM to the state the end to end tests need.

Creates the first user, scans the library that was mounted in, mints a client
token and registers a device. Prints the token and the device id on stdout,
one per line, so the caller can hand them to grout.

Everything here goes through RomM's own API rather than its database, so a
change in how RomM stores things does not quietly break the tests, and a
change in its API breaks them loudly, which is the point.
"""

import os
import sys

import requests
import socketio

URL = os.environ.get("ROMM_URL", "http://localhost:8080")
USER = os.environ.get("ROMM_USER", "e2e")
PASSWORD = os.environ.get("ROMM_PASSWORD", "e2e-password")

# What grout asks for when it pairs. Anything missing here shows up as a
# permission error deep in a sync rather than as a setup failure.
SCOPES = [
    "me.read", "platforms.read", "roms.read", "collections.read",
    "firmware.read", "assets.read", "assets.write",
    "devices.read", "devices.write",
]


def session_with_csrf() -> tuple[requests.Session, str, dict]:
    """A session holding the CSRF cookie RomM checks every write against."""
    session = requests.Session()
    heartbeat = session.get(f"{URL}/api/heartbeat", timeout=30)
    heartbeat.raise_for_status()
    return session, session.cookies.get("romm_csrftoken", ""), heartbeat.json()


def create_first_user(session: requests.Session, csrf: str) -> None:
    """Create the admin RomM asks for before it will do anything.

    Only possible while RomM is still offering its setup wizard; afterwards
    the endpoint needs the very credentials it would be creating. Callers
    check the wizard flag first.
    """
    response = session.post(
        f"{URL}/api/users",
        json={
            "username": USER,
            "password": PASSWORD,
            "email": f"{USER}@example.invalid",
            "role": "admin",
        },
        headers={"X-CSRFToken": csrf},
        timeout=30,
    )
    response.raise_for_status()


def scan_library(session: requests.Session, csrf: str) -> None:
    """Ask RomM to read the library into its database.

    A scan is only offered over the websocket, so this does what the web UI
    does. The handshake is authorised by the session cookie; the basic auth
    the REST API takes is not enough for it.
    """
    session.post(
        f"{URL}/api/login", auth=(USER, PASSWORD),
        headers={"X-CSRFToken": csrf}, timeout=30,
    ).raise_for_status()

    cookies = "; ".join(f"{k}={v}" for k, v in session.cookies.items())

    client = socketio.Client()
    outcome: dict[str, str | bool] = {"done": False, "error": ""}

    @client.on("scan:done")
    def _finished(*_args):
        outcome["done"] = True
        client.disconnect()

    @client.on("scan:done_ko")
    def _failed(message=""):
        outcome["error"] = str(message)
        client.disconnect()

    client.connect(
        URL,
        headers={"Cookie": cookies, "X-CSRFToken": csrf},
        socketio_path="/ws/socket.io",
        wait_timeout=60,
    )
    # No metadata sources: the library here is made up, and reaching out to a
    # third party would make the tests depend on one being up.
    client.emit("scan", {"platforms": [], "type": "quick", "apis": []})
    client.wait()

    if not outcome["done"]:
        raise RuntimeError(f"scan did not finish: {outcome['error']}")


def register_device(session: requests.Session, csrf: str) -> str:
    """Register a device, so save sync has one to sync to.

    Grout registers its own through the same endpoint when a person walks the
    pairing flow. Doing it here lets a test start with a card that is already
    known to the server.
    """
    response = session.post(
        f"{URL}/api/devices",
        auth=(USER, PASSWORD),
        json={
            "name": "grout-e2e-device",
            "platform": "e2e",
            "client": "grout",
            "sync_mode": "api",
            "allow_existing": True,
        },
        headers={"X-CSRFToken": csrf},
        timeout=30,
    )
    response.raise_for_status()
    return response.json()["device_id"]


def mint_token(session: requests.Session, csrf: str) -> str:
    response = session.post(
        f"{URL}/api/client-tokens",
        auth=(USER, PASSWORD),
        json={"name": "grout-e2e", "scopes": SCOPES},
        headers={"X-CSRFToken": csrf},
        timeout=30,
    )
    response.raise_for_status()
    return response.json()["raw_token"]


def main() -> int:
    session, csrf, heartbeat = session_with_csrf()

    # RomM says whether it still wants a first user. Asking it beats guessing
    # from a status code, and makes a re-run against a server that is already
    # set up do the right thing.
    if heartbeat.get("SYSTEM", {}).get("SHOW_SETUP_WIZARD"):
        create_first_user(session, csrf)

    scan_library(session, csrf)

    platforms = session.get(f"{URL}/api/platforms", auth=(USER, PASSWORD), timeout=30).json()
    if not platforms:
        print("the scan found no platforms; is the library mounted?", file=sys.stderr)
        return 1
    print(f"scanned {len(platforms)} platforms", file=sys.stderr)

    # One line each, so the caller can read them without parsing anything.
    print(mint_token(session, csrf))
    print(register_device(session, csrf))
    return 0


if __name__ == "__main__":
    sys.exit(main())
