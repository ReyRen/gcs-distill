"""Acceptance-only OpenAI-compatible teacher using local Qwen2-VL weights."""
import argparse
import base64
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import io
import json
from pathlib import Path
import threading
import time

from PIL import Image, ImageDraw
import torch
from transformers import AutoProcessor, Qwen2VLForConditionalGeneration

parser = argparse.ArgumentParser(description='Acceptance only; use the model center for production serving.')
parser.add_argument('--model-path', default='/model')
parser.add_argument('--image-dir', type=Path, required=True)
parser.add_argument('--port', type=int, default=18883)
args = parser.parse_args()
model_id = 'Qwen2-VL-2B-Instruct'
model_path = args.model_path
processor = AutoProcessor.from_pretrained(model_path, local_files_only=True, min_pixels=56 * 56, max_pixels=224 * 224)
model = Qwen2VLForConditionalGeneration.from_pretrained(model_path, local_files_only=True,
    torch_dtype=torch.float16, attn_implementation='sdpa').to('cuda').eval()
lock = threading.Lock()
asset_dir = args.image_dir
asset_dir.mkdir(parents=True, exist_ok=True)
for name, color, shape in [('red-square', 'red', 'square'), ('blue-circle', 'blue', 'circle')]:
    image = Image.new('RGB', (224, 224), 'white')
    draw = ImageDraw.Draw(image)
    if shape == 'square':
        draw.rectangle((45, 45, 179, 179), fill=color)
    else:
        draw.ellipse((45, 45, 179, 179), fill=color)
    image.save(asset_dir / (name + '.png'))


class Handler(BaseHTTPRequestHandler):
    def send_json(self, status, value):
        data = json.dumps(value).encode()
        self.send_response(status)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        if self.path == '/v1/models':
            self.send_json(200, {'object': 'list', 'data': [{'id': model_id, 'object': 'model'}]})
        elif self.path == '/health':
            self.send_json(200, {'ready': True, 'model': model_id, 'weights_source': 'local'})
        else:
            self.send_json(404, {'error': 'not found'})

    def do_POST(self):
        if self.path != '/v1/chat/completions':
            self.send_json(404, {'error': 'not found'})
            return
        if self.headers.get('Authorization') != 'Bearer acceptance':
            self.send_json(401, {'error': 'authorization required'})
            return
        try:
            request = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
            messages = []
            images = []
            for message in request['messages']:
                content = message['content']
                if isinstance(content, list):
                    converted = []
                    for part in content:
                        if part['type'] == 'image_url':
                            reference = part['image_url']['url']
                            if not reference.startswith('data:image/'):
                                raise ValueError('Acceptance teacher only accepts embedded images')
                            image = Image.open(io.BytesIO(base64.b64decode(reference.split(',', 1)[1]))).convert('RGB')
                            images.append(image)
                            converted.append({'type': 'image', 'image': image})
                        elif part['type'] == 'text':
                            converted.append(part)
                        else:
                            raise ValueError('unsupported content type')
                    content = converted
                messages.append({'role': message['role'], 'content': content})
            prompt = processor.apply_chat_template(messages, tokenize=False, add_generation_prompt=True)
            with lock, torch.inference_mode():
                inputs = processor(text=[prompt], images=images or None, padding=True, return_tensors='pt').to('cuda')
                outputs = model.generate(**inputs, max_new_tokens=min(int(request.get('max_tokens', 256)), 1024), do_sample=False)
                answer_tokens = outputs[:, inputs.input_ids.shape[1]:]
                answer = processor.batch_decode(answer_tokens, skip_special_tokens=True)[0]
            print('VISION_COMPLETION', json.dumps({'images': len(images), 'answer': answer}), flush=True)
            self.send_json(200, {'id': 'vision-acceptance-' + str(time.time_ns()), 'object': 'chat.completion',
                'model': model_id, 'choices': [{'index': 0, 'message': {'role': 'assistant', 'content': answer}, 'finish_reason': 'stop'}],
                'usage': {'prompt_tokens': inputs.input_ids.shape[1], 'completion_tokens': answer_tokens.shape[1],
                          'total_tokens': outputs.shape[1]}})
        except Exception as error:
            self.send_json(500, {'error': str(error)})


print('VISION_TEACHER_READY', model_id, flush=True)
ThreadingHTTPServer(('0.0.0.0', args.port), Handler).serve_forever()
