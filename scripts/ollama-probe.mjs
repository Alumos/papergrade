// Explicit real-network OCR probe. Nothing is called automatically in CI.
// Usage: OLLAMA_BASE_URL=http://NAS:11434 node scripts/ollama-probe.mjs /path/image.png
import fs from 'node:fs/promises';
import process from 'node:process';
const base=(process.env.OLLAMA_BASE_URL??'').replace(/\/$/,'');
const model=process.env.OLLAMA_MODEL??'longwayxu/xiaomi-ocr-0:bf16';
const image=process.argv[2];
if(!base||!image)throw Error('Set OLLAMA_BASE_URL and pass a local PNG/JPEG/WebP image path. Optional OLLAMA_API_KEY/OLLAMA_MODEL.');
const headers={'Content-Type':'application/json'};
if(process.env.OLLAMA_API_KEY)headers.Authorization='Bearer '+process.env.OLLAMA_API_KEY;
const bytes=await fs.readFile(image);if(bytes.length>4*1024*1024)throw Error('Image exceeds4MiB; resize before requesting');
const request={model,stream:false,think:false,messages:[{role:'user',content:'Extract all information from the main body of the document image and represent it in markdown format, ignoring headers and footers. Tables should be expressed in OTSL format, formulas in the document should be represented using LATEX format, and the parsing should be organized according to the reading order.',images:[bytes.toString('base64')]}],options:{temperature:0,num_ctx:8192,num_predict:4096}};
const start=performance.now();const res=await fetch(base+'/api/chat',{method:'POST',headers,body:JSON.stringify(request),redirect:'error',signal:AbortSignal.timeout(300000)});
if(!res.ok)throw Error('Ollama HTTP '+res.status+'; inspect NAS logs locally (response body omitted)');
const data=await res.json();if(data.error)throw Error('Ollama returned an error; inspect NAS logs');
const text=data.message?.content;if(!text||data.done_reason==='length')throw Error('Empty or truncated OCR result');
console.log(text);console.error(JSON.stringify({model:data.model,seconds:Math.round((performance.now()-start)/10)/100,done_reason:data.done_reason,prompt_tokens:data.prompt_eval_count,output_tokens:data.eval_count}));
