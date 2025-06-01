# Owntracks2HA

`owntracks2ha` is a lightweight MQTT bridge that listens to [OwnTracks](https://owntracks.org/) messages from one MQTT broker and republishes them to another MQTT broker in a format compatible with [Home Assistant](https://www.home-assistant.io/).

---

## 🔧 Configuration

Create `config/config.yaml` with the following structure:

```yaml
source_broker: "mqtt1.example.com"
source_port: 1883
source_user: "source_owntracks2ha_user"
source_pass: "source_owntracks2ha_pass"

target_broker: "mqtt2.example.com"
target_port: 1883
target_user: "target_owntracks2ha_user"
target_pass: "target_owntracks2ha_pass"

use_tls: false
qos: 1
run_mode: "daemon"

mappings:
  owntracks/user1/device1: owntracks_converted/user1/device1
