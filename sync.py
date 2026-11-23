import os, hashlib, time, requests

url = "https://lumiflex.ru"
params = {"rppId": "da9377c8d40e941b5926b3ded7d1e531"}
ns_url = os.environ["NIGHTSCOUT_URL"].rstrip("/")
ns_secret = hashlib.sha1(os.environ["NIGHTSCOUT_SECRET"].encode()).hexdigest()

headers = {
    "Accept": "application/json, text/plain, */*",
    "Token": os.environ["LUMIFLEX_TOKEN"],
    "Time-Zone": "Europe/Moscow",
    "Accept-Language": "ru",
    "Version": "eu"
}

resp = requests.get(url, params=params, headers=headers, timeout=15).json()
print("Lumiflex response:", resp)

data = resp.get("data") or {}
glucose_mmol = data.get("glucose")

if glucose_mmol:
    sgv = round(float(glucose_mmol) * 18.0182)
    entry = [{
        "type": "sgv",
        "sgv": sgv,
        "direction": "Flat",
        "date": int(time.time() * 1000),
        "dateString": time.strftime("%Y-%m-%dT%H:%M:%S.000Z", time.gmtime())
    }]
    r = requests.post(
        ns_url + "/api/v1/entries",
        json=entry,
        headers={"API-SECRET": ns_secret, "Content-Type": "application/json"},
        timeout=15
    )
    print("Nightscout status:", r.status_code, r.text)
