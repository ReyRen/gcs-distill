#!/usr/bin/env python3
"""Remove only obsolete distillation Docker resources; never run system prune."""
import argparse
import json
import subprocess

parser = argparse.ArgumentParser()
parser.add_argument('--apply', action='store_true')
parser.add_argument('--include-previous-v2', action='store_true')
args = parser.parse_args()
tags = ['gcs-distill/easydistill:latest', 'easy-distill/easydistill:latest',
        'gcs-distill-gcs-worker-1:latest', 'gcs-distill-gcs-server:latest']
if args.include_previous_v2:
    tags.append('gcs/distill-easydistill:2.0.0-aeb8ca3-x86_64')

def docker(*items):
    return subprocess.run(['docker'] + list(items), stdout=subprocess.PIPE,
                          stderr=subprocess.PIPE, universal_newlines=True)

def containers():
    ids = docker('ps', '-aq').stdout.split()
    if not ids:
        return []
    result = docker('inspect', *ids)
    if result.returncode:
        raise RuntimeError(result.stderr)
    return json.loads(result.stdout)

def owned(c):
    labels = c['Config'].get('Labels') or {}
    return (labels.get('com.docker.compose.project') == 'gcs-distill'
            or labels.get('gcs.application') == 'gcs-distill'
            or (c['Name'].lstrip('/').startswith('distill-') and
                any('/train-center/model-distill/' in m.get('Source', '') for m in c.get('Mounts', []))))

report = {'apply': args.apply, 'containers': [], 'images': [], 'volumes': [], 'blocked': []}
for tag in tags:
    image = docker('image', 'inspect', tag)
    if image.returncode:
        continue
    image_id = json.loads(image.stdout)[0]['Id']
    refs = [c for c in containers() if c['Image'] == image_id]
    others = [c['Name'] for c in refs if not owned(c)]
    if others:
        report['blocked'].append({'image': tag, 'other_project_containers': others})
        continue
    for c in refs:
        report['containers'].append({'name': c['Name'], 'id': c['Id'], 'state': c['State']['Status']})
        volumes = [m.get('Name') for m in c.get('Mounts', [])
                   if m.get('Type') == 'volume' and m.get('Destination') == '/var/lib/docker']
        if args.apply:
            result = docker('rm', '-f', c['Id'])
            if result.returncode:
                raise RuntimeError(result.stderr)
            for volume in volumes:
                in_use = any(m.get('Name') == volume for other in containers() for m in other.get('Mounts', []))
                if volume and not in_use:
                    result = docker('volume', 'rm', volume)
                    if result.returncode:
                        raise RuntimeError(result.stderr)
                    report['volumes'].append(volume)
    if args.apply:
        if any(c['Image'] == image_id for c in containers()):
            report['blocked'].append({'image': tag, 'reason': 'new container reference appeared'})
            continue
        result = docker('image', 'rm', tag)
        if result.returncode:
            raise RuntimeError(result.stderr)
    report['images'].append({'tag': tag, 'id': image_id})
print(json.dumps(report, indent=2))
