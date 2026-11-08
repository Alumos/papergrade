// LEGACY v0.1 script retained as historical reference; new releases MUST run
// scripts/v02-e2e.mjs (compiled card-fixture and db-sandbox required).
// This old fixture does not create valid a5-v2 student-number evidence.
// Disposable integration test only. Starts isolated API+Worker and a controlled
// protocol fixture cloud, exercising the real PostgreSQL queue and persistence.
import assert from 'node:assert/strict';
import { spawn, execFileSync } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { createServer } from 'node:http';
import { deflateSync } from 'node:zlib';
const database=process.env.TEST_DATABASE_URL;
if(!database)throw Error('TEST_DATABASE_URL must point to a disposable PostgreSQL database');
const binary=process.env.PAPERGRADE_BINARY ?? '.cache/papergrade-dev';
const base='http://127.0.0.1:8082',origin='https://integration.invalid';
let token='',qid='',cookie='',cloudPath='',cloudKey='';
const cloud=createServer((req,res)=>{cloudPath=req.url;cloudKey=req.headers.authorization??'';req.resume();req.on('end',()=>{res.setHeader('Content-Type','application/json');res.end(JSON.stringify({token,answers:[{question_id:qid,selected:['A'],state:'answered',confidence:.99}]}))});});
await new Promise(r=>cloud.listen(0,'127.0.0.1',r));
const env={...process.env,APP_ENCRYPTION_KEY:'disposable-integration-only-master-key-not-production',DATABASE_URL:database,ADMIN_USERNAME:'e2e-only',ADMIN_PASSWORD:randomBytes(24).toString('hex'),PUBLIC_ORIGIN:origin,DEV_MODE:'false',DATA_DIR:'.cache/e2e-data',SERVER_ADDR:'127.0.0.1:8082',CARD_ENDPOINT:'',CARD_API_KEY:'',OCR_ENDPOINT:'',OCR_API_KEY:'',AI_BASE_URL:'',AI_API_KEY:'',AI_MODEL:'',DB_MAX_CONNS:'2'};
const children=[];
function start(command){const p=spawn(binary,[command],{env,stdio:['ignore','ignore','pipe']});p.stderr.on('data',b=>{if(String(b).includes('ERROR'))process.stderr.write(b)});children.push(p);return p;}
async function until(fn,limit=15000){const deadline=Date.now()+limit;let last;while(Date.now()<deadline){try{const v=await fn();if(v)return v;}catch(e){last=e;}await new Promise(r=>setTimeout(r,100));}throw last??Error('condition timeout');}
async function req(path,method='GET',body,extra={}){const headers={Origin:origin,...extra};if(cookie)headers.Cookie=cookie;let payload=body;if(body&&!(body instanceof FormData)){headers['Content-Type']='application/json';payload=JSON.stringify(body)}const r=await fetch(base+path,{method,headers,body:payload});const c=r.headers.get('set-cookie');if(c)cookie=c.split(';')[0];const text=await r.text();let data;try{data=JSON.parse(text)}catch{data=text}if(!r.ok)throw Error(method+' '+path+' '+r.status+': '+text);return{status:r.status,data};}
function png(){const crc=b=>{let c=-1;for(const v of b){c^=v;for(let i=0;i<8;i++)c=(c>>>1)^((c&1)?0xedb88320:0);}return(c^-1)>>>0;};const chunk=(name,b)=>{const t=Buffer.from(name),n=Buffer.alloc(4),c=Buffer.alloc(4);n.writeUInt32BE(b.length);c.writeUInt32BE(crc(Buffer.concat([t,b])));return Buffer.concat([n,t,b,c]);};const ihdr=Buffer.alloc(13);ihdr.writeUInt32BE(100,0);ihdr.writeUInt32BE(100,4);ihdr[8]=8;return Buffer.concat([Buffer.from([137,80,78,71,13,10,26,10]),chunk('IHDR',ihdr),chunk('IDAT',deflateSync(Buffer.alloc(101*100,0))),chunk('IEND',Buffer.alloc(0))]);}
let api,worker;
try{
 execFileSync(binary,['migrate'],{env,stdio:'pipe'});api=start('api');await until(async()=>{const r=await fetch(base+'/readyz');return r.ok});worker=start('worker');
 await req('/api/v1/auth/login','POST',{username:env.ADMIN_USERNAME,password:env.ADMIN_PASSWORD});
 let settings=(await req('/api/v1/settings/providers')).data;
 const providerBody={revision:settings.revision,ocr:{url:''},ai:{base_url:'',model:''},card:{url:'http://127.0.0.1:'+cloud.address().port+'/initial',secret:'fixture-only-card-key'}};
 settings=(await req('/api/v1/settings/providers','PUT',providerBody)).data;assert.equal(settings.source,'database');assert.equal(settings.card.has_secret,true);assert(!JSON.stringify(settings).includes('fixture-only-card-key'));
 providerBody.revision=settings.revision;providerBody.card={url:'http://127.0.0.1:'+cloud.address().port+'/updated',secret:''};
 settings=(await req('/api/v1/settings/providers','PUT',providerBody)).data;assert.equal(settings.card.has_secret,true);
 const cls=(await req('/api/v1/classes','POST',{name:'PG E2E '+Date.now()})).data;
 await req('/api/v1/classes/'+cls.id+'/students','POST',{students:[{number:'0001',name:'集成测试学生'}]});
 let ex=(await req('/api/v1/exams','POST',{title:'Persistent queue E2E',subject:'integration',class_id:cls.id,questions:[{number:'1',type:'single_choice',options:['A','B'],answer:['A'],score_units:200}]})).data;
 ex=(await req('/api/v1/exams/'+ex.id+'/publish','POST',{revision:ex.revision})).data;
 qid=ex.questions[0].id;token=(await req('/api/v1/exams/'+ex.id+'/sheets')).data.items[0].token;
 const id=randomBytes(16).toString('hex'),form=new FormData();form.set('image',new Blob([png()],{type:'image/png'}),'test.png');form.set('client_capture_id',id);form.set('candidate_sheet_token',token);
 const accepted=await req('/api/v1/exams/'+ex.id+'/captures','POST',form,{'Idempotency-Key':id});assert.equal(accepted.status,202);
 const result=await until(async()=>{const r=(await req('/api/v1/captures/'+accepted.data.capture_id)).data;if(r.status==='failed')throw Error(JSON.stringify(r));return r.status==='committed'?r:false});assert.equal(result.score_units,200);assert.equal(cloudPath,'/updated');assert.equal(cloudKey,'Bearer fixture-only-card-key');
 const repeat=(await req('/api/v1/exams/'+ex.id+'/captures','POST',form,{'Idempotency-Key':id})).data;assert.equal(repeat.capture_id,result.capture_id);assert.equal(repeat.deduplicated,true);
 const corrected=(await req('/api/v1/exams/'+ex.id+'/regrade','POST',{revision:ex.revision,answers:[{question_id:qid,answer:['B']}],reason:'integration-only corrected answer'})).data;assert.equal(corrected.updated_results,1);
 let results=(await req('/api/v1/exams/'+ex.id+'/results')).data;assert.equal(results.summary.received,1);assert.equal(results.items[0].score_units,0);
 api.kill('SIGTERM');await new Promise(r=>api.once('exit',r));api=start('api');await until(async()=>{const r=await fetch(base+'/readyz');return r.ok});
 settings=(await req('/api/v1/settings/providers')).data;assert.equal(settings.source,'database');assert.equal(settings.card.has_secret,true);assert(settings.card.url.endsWith('/updated'));
 results=(await req('/api/v1/exams/'+ex.id+'/results')).data;assert.equal(results.items[0].score_units,0);
 console.log('PASS PostgreSQL E2E: encrypted settings / masked and retained key / live Worker config update / restart decryption / secure-cookie auth / persistent upload / 202 queue / independent cloud protocol fixture / Worker commit / idempotency / versioned regrade / API restart persistence. NOT real cloud OMR accuracy.');
}finally{
 for(const p of children)if(p.exitCode===null){p.kill('SIGTERM');await Promise.race([new Promise(r=>p.once('exit',r)),new Promise(r=>setTimeout(r,3000))]);if(p.exitCode===null)p.kill('SIGKILL');}
 await new Promise(r=>cloud.close(r));
}
