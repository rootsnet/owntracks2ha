# owntracks2ha

`owntracks2ha` is a lightweight MQTT bridge that listens to [OwnTracks](https://owntracks.org/) messages from one MQTT broker and republishes them to another MQTT broker in a format compatible with [Home Assistant](https://www.home-assistant.io/).

---

## 🔧 Configuration

Create `config/config.yaml` with the following structure:

```yaml
source_broker: "mqtt1.example.com"
source_port: 1883
source_user: "user1"
source_pass: "pass1"

target_broker: "mqtt2.example.com"
target_port: 1883
target_user: "user2"
target_pass: "pass2"

use_tls: false
qos: 1
run_mode: "daemon"

mappings:
  owntracks/user1/device1: owntracks_converted/user1/device1
