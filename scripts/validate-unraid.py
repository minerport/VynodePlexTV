"""Structural release checks; CA's hosted Validate/Scan is still required."""
from pathlib import Path
import xml.etree.ElementTree as ET

root = Path(__file__).resolve().parent.parent
app = ET.parse(root / 'templates/afterglow-tv.xml').getroot()
assert app.tag == 'Container' and app.get('version') == '2'
for field in ['Name', 'Repository', 'Network', 'WebUI', 'Project', 'TemplateURL', 'Icon', 'License']:
    assert app.findtext(field), field
assert app.findtext('Privileged') == 'false'
assert app.findtext('Repository') == 'ghcr.io/minerport/vynodeplextv:latest'
assert app.findtext('TemplateURL').endswith('/main/templates/afterglow-tv.xml')
config = {node.get('Target'): node for node in app.findall('Config')}
assert config['/config'].get('Type') == 'Path'
assert config['8080'].get('Type') == 'Port'
assert config['APP_ORIGIN'].get('Required') == 'true'
assert '--user=99:100' in app.findtext('ExtraParams')
assert 'PUID' not in config and 'PGID' not in config
assert ET.parse(root / 'ca_profile.xml').getroot().findtext('Profile')
assert (root / 'LICENSE').is_file()
print('Unraid template/profile structural checks passed')

# Keep manual installs consistent with template installs.
dockerfile = (root / 'Dockerfile').read_text()
assert f'net.unraid.docker.icon="{app.findtext("Icon")}"' in dockerfile
assert f'net.unraid.docker.webui="{app.findtext("WebUI")}"' in dockerfile

# Dockerman does not reliably render SVG icons. Verify a real PNG, not a renamed SVG.
import struct
icon = (root / 'icon.png').read_bytes()
assert app.findtext('Icon').endswith('/icon.png')
assert ET.parse(root / 'ca_profile.xml').getroot().findtext('Icon') == app.findtext('Icon')
assert icon[:8] == b'\x89PNG\r\n\x1a\n'
assert struct.unpack('>II', icon[16:24]) == (256, 256)
