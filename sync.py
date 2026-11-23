import os, hashlib, time, requests

RPP_ID = "da9377c8d40e941b5926b3ded7d1e531"
URL = f"https://lumiflex.ru{RPP_ID}"
NS_URL = os.environ["NIGHTSCOUT_URL"].rstrip("/")
NS_SECRET = hashlib.sha1(os.environ["NIGHTSCOUT_SECRET"].encode()).hexdigest()

headers = {
    "Accept": "application/json, text/plain, */*",
    "Token": os.environ["LUMIFLEX_TOKEN"],
    "Time-Zone": "Europe/Moscow",
    "Accept-Language": "ru",
    "Version": "eu"
}

resp = requests.get(URL, headers=headers, timeout=15).json()
print("Lumiflex response:", resp)

data = resp.get("data") or {}
glucose_mmol = data.get("glucose")

if glucose_mmol:
    sgv = round(float(glucose_mmol) * 18.0182)
    now_ms = int(time.time() * 1000)
    entry = [{
        "type": "sgv",
        "sgv": sgv,
        "direction": "Flat",
        "date": now_ms,
        "dateString": time.strftime("%Y-%m-%dT%H:%M:%S.000Z", time.gmtime())
    }]
    r = requests.post(
        f"{NS_URL}/api/v1/entries",
        json=entry,
        headers={"API-SECRET": NS_SECRET, "Content-Type": "application/json"},
        timeout=15
    )
    print("Nightscout status:", r.status_code, r.text)
