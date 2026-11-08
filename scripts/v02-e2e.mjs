// Disposable v0.2 E2E: real PostgreSQL/processes, native Ollama protocol, AI fixture,
// teacher confirmation, actual deterministic student-number OMR and A4 two-up.
// REAL_OLLAMA_URL and REAL_OCR_IMAGE opt into a real authorized NAS request.
import assert from 'node:assert/strict';
import {spawn,execFileSync} from 'node:child_process';
import {randomBytes} from 'node:crypto';
import {createServer} from 'node:http';
import fs from 'node:fs/promises';
import path from 'node:path';
import {deflateSync} from 'node:zlib';
let db=process.env.TEST_DATABASE_URL;if(!db)throw Error('Use a disposable TEST_DATABASE_URL');
const sandboxBinary=process.env.DB_SANDBOX_BINARY??'.cache/db-sandbox';
const schema='v02_'+randomBytes(8).toString('hex');
db=execFileSync(sandboxBinary,['-create',schema],{encoding:'utf8'});
const binary=process.env.PAPERGRADE_BINARY??'.cache/papergrade-dev';
const fixtureBinary=process.env.CARD_FIXTURE_BINARY??'.cache/card-fixture';
const base='http://127.0.0.1:8083',origin='https://v02.integration.invalid';
const output=path.resolve('.cache/v02-e2e');await fs.mkdir(output,{recursive:true});
const q={number:'5',type:'single_choice',options:['A','B','C','D'],answer:[],score_units:200,source_page:1,review_flags:['missing_answer']};
let ocrCalls=0,aiCalls=0;
const fixture=createServer(async(req,res)=>{let body='';for await(const chunk of req)body+=chunk;res.setHeader('Content-Type','application/json');
 if(req.url==='/api/chat'){const data=JSON.parse(body);assert(data.messages[0].images[0]);assert.equal(data.stream,false);assert.equal(data.think,false);ocrCalls++;res.end(JSON.stringify({model:data.model,done:true,done_reason:'stop',message:{content:'5. 函数 f(x)=\\frac{x^2+2x+3}{x^2+4x+5} 的最大值与最小值的和为 A.1 B.2 C.3 D.4'}}));}
 else if(req.url==='/v1/chat/completions'){const data=JSON.parse(body);assert(data.messages.at(-1).content.includes('5.'));aiCalls++;res.end(JSON.stringify({choices:[{message:{content:JSON.stringify({questions:[q],warnings:['teacher confirmation required']})}}],usage:{prompt_tokens:100,completion_tokens:80}}));}
 else if(req.url==='/v1/models'){res.end(JSON.stringify({data:[{id:'fixture-model'}]}));}
 else{res.writeHead(404);res.end('{}');}
});await new Promise(r=>fixture.listen(0,'127.0.0.1',r));const fake='http://127.0.0.1:'+fixture.address().port;
const key='v02-disposable-settings-master-key-test-only';
const env={...process.env,APP_ENCRYPTION_KEY:key,DATABASE_URL:db,ADMIN_USERNAME:'v02-test',ADMIN_PASSWORD:randomBytes(24).toString('hex'),PUBLIC_ORIGIN:origin,DEV_MODE:'false',DATA_DIR:output,SERVER_ADDR:'127.0.0.1:8083',DB_MAX_CONNS:'2',OCR_ENDPOINT:'',OCR_API_KEY:'',AI_BASE_URL:'',AI_API_KEY:'',AI_MODEL:'',CARD_ENDPOINT:'',CARD_API_KEY:''};
const children=[];let cookie='';
function start(role){const p=spawn(binary,[role],{env,stdio:['ignore','ignore','pipe']});p.stderr.on('data',b=>{if(String(b).includes('ERROR'))process.stderr.write(b)});children.push(p);return p;}
async function until(fn,ms=30000){const end=Date.now()+ms;let last;while(Date.now()<end){try{const v=await fn();if(v)return v;}catch(e){last=e;}await new Promise(r=>setTimeout(r,200));}throw last??Error('condition timeout');}
async function request(route,method='GET',data,extra={},expected){const headers={Origin:origin,...extra};if(cookie)headers.Cookie=cookie;let body=data;if(data&&!(data instanceof FormData)){headers['Content-Type']='application/json';body=JSON.stringify(data)}const r=await fetch(base+route,{method,headers,body});const set=r.headers.get('set-cookie');if(set)cookie=set.split(';')[0];const text=await r.text();let result;try{result=JSON.parse(text)}catch{result=text}if(expected!==undefined){assert.equal(r.status,expected,text);}else if(!r.ok)throw Error(method+' '+route+' '+r.status+' '+text);return result;}
function makePNG(){const crc=b=>{let c=-1;for(const v of b){c^=v;for(let i=0;i<8;i++)c=(c>>>1)^((c&1)?0xedb88320:0)}return(c^-1)>>>0};const chunk=(n,b)=>{const name=Buffer.from(n),size=Buffer.alloc(4),checksum=Buffer.alloc(4);size.writeUInt32BE(b.length);checksum.writeUInt32BE(crc(Buffer.concat([name,b])));return Buffer.concat([size,name,b,checksum])};const h=Buffer.alloc(13);h.writeUInt32BE(200);h.writeUInt32BE(300,4);h[8]=8;return Buffer.concat([Buffer.from([137,80,78,71,13,10,26,10]),chunk('IHDR',h),chunk('IDAT',deflateSync(Buffer.alloc(201*300,0))),chunk('IEND',Buffer.alloc(0))]);}
let api,worker;
try{
 execFileSync(binary,['migrate'],{env,stdio:'pipe'});api=start('api');await until(async()=>{const r=await fetch(base+'/readyz');return r.ok});worker=start('worker');
 await request('/api/v1/auth/login','POST',{username:env.ADMIN_USERNAME,password:env.ADMIN_PASSWORD});
 let settings=await request('/api/v1/settings/providers');
 settings=await request('/api/v1/settings/providers','PUT',{revision:settings.revision,ocr:{type:'ollama',url:process.env.REAL_OLLAMA_URL??fake,model:process.env.REAL_OLLAMA_MODEL??'longwayxu/xiaomi-ocr-0:bf16',secret:'',clear_secret:true},ai:{base_url:fake+'/v1',model:'fixture-model',secret:'fixture-not-a-real-key'},card:{url:''}});
 assert.equal(settings.ocr.type,'ollama');assert.equal(settings.ocr.has_secret,false);
 const cls=await request('/api/v1/classes','POST',{name:'V02 '+Date.now()});await request('/api/v1/classes/'+cls.id+'/students','POST',{students:[{number:'0001',name:'测试学生一'},{number:'0002',name:'测试学生二'}]});
 let exam=await request('/api/v1/exams','POST',{title:'数学公式入库与学号OMR',subject:'数学',class_id:cls.id,questions:[]});
 let image=makePNG();let filename='source.png';if(process.env.REAL_OCR_IMAGE){image=await fs.readFile(process.env.REAL_OCR_IMAGE);filename=path.basename(process.env.REAL_OCR_IMAGE)}
 const upload=new FormData();upload.set('file',new Blob([image],{type:'image/png'}),filename);
 const document=await request('/api/v1/exams/'+exam.id+'/documents','POST',upload);await request('/api/v1/exams/'+exam.id+'/parse','POST',{document_id:document.id});
 const parsed=await until(async()=>{const j=await request('/api/v1/exams/'+exam.id+'/documents');const d=j.items.find(x=>x.id===document.id);if(d.status==='failed')throw Error(JSON.stringify(d.page_errors));return d.status==='parsed'?d:false},process.env.REAL_OLLAMA_URL?330000:30000);
 assert.equal(parsed.progress,100);assert(parsed.artifact.extraction.text.includes('frac'));assert.equal(parsed.artifact.analysis.questions[0].number,'5');
 const adopted=await request('/api/v1/exams/'+exam.id+'/documents/'+document.id+'/adopt','POST',{revision:exam.revision,replace_existing:false});exam=adopted.exam;assert.equal(exam.questions[0].answer.length,0);
 await request('/api/v1/exams/'+exam.id+'/publish','POST',{revision:exam.revision},{},400);
 exam=await request('/api/v1/exams/'+exam.id,'PATCH',{title:exam.title,subject:exam.subject,class_id:cls.id,revision:exam.revision,questions:exam.questions.map(x=>({...x,answer:['B']}))});
 exam=await request('/api/v1/exams/'+exam.id+'/publish','POST',{revision:exam.revision});
 assert.equal(exam.template.version,'a5-v2');assert.equal(exam.template.height_mm,148.5);assert.equal(exam.student_number_digits,4);
 const roster=await request('/api/v1/classes/'+cls.id+'/students');const sheets=await request('/api/v1/exams/'+exam.id+'/sheets');assert.equal(sheets.items[0].token,sheets.items[1].token);
 const svg=await request('/api/v1/exams/'+exam.id+'/print-sheet?student_ids='+roster.items.map(x=>x.id).join(','));assert(svg.includes('height="297mm"'));assert(svg.includes('148.500'));await fs.writeFile(path.join(output,'two-up.svg'),svg);await fs.writeFile(path.join(output,'manifest.json'),JSON.stringify(exam.template));await fs.writeFile(path.join(output,'source.ocr.md'),parsed.artifact.extraction.text);
 const qr=sheets.items[0].token;
 async function scan(number){const out=path.join(output,'card-'+number+'.png');execFileSync(fixtureBinary,['-manifest',path.join(output,'manifest.json'),'-number',number,'-token',qr,'-answers','B','-output',out],{stdio:'pipe'});const form=new FormData(),id=randomBytes(16).toString('hex');form.set('image',new Blob([await fs.readFile(out)],{type:'image/png'}),'card.png');form.set('client_capture_id',id);form.set('candidate_sheet_token',qr);const accepted=await request('/api/v1/exams/'+exam.id+'/captures','POST',form,{'Idempotency-Key':id});const result=await until(async()=>{const c=await request('/api/v1/captures/'+accepted.capture_id);if(c.status==='failed')throw Error(c.message);return ['committed','needs_review','conflict','duplicate'].includes(c.status)?c:false});return{result,form,id};}
 const one=await scan('0001');assert.equal(one.result.status,'committed',JSON.stringify(one.result));assert.equal(one.result.student_number,'0001');assert.equal(one.result.score_units,200);
 const two=await scan('0002');assert.equal(two.result.status,'committed',JSON.stringify(two.result));assert.equal(two.result.student_number,'0002');
 const retry=await request('/api/v1/exams/'+exam.id+'/captures','POST',one.form,{'Idempotency-Key':one.id});assert.equal(retry.capture_id,one.result.capture_id);
 let results=await request('/api/v1/exams/'+exam.id+'/results');assert.equal(results.summary.received,2);
 const revised=await request('/api/v1/exams/'+exam.id+'/regrade','POST',{revision:exam.revision,answers:exam.questions.map(x=>({question_id:x.id,answer:['A']})),reason:'E2E teacher key correction'});assert.equal(revised.updated_results,2);
 results=await request('/api/v1/exams/'+exam.id+'/results');assert(results.items.every(x=>x.score_units===0));
 api.kill('SIGTERM');await new Promise(r=>api.once('exit',r));api=start('api');await until(async()=>{const r=await fetch(base+'/readyz');return r.ok});
 const restored=await request('/api/v1/settings/providers');assert.equal(restored.source,'database');assert.equal(restored.ocr.type,'ollama');
 results=await request('/api/v1/exams/'+exam.id+'/results');assert.equal(results.summary.received,2);assert(results.items.every(x=>x.score_units===0));
 await fs.writeFile(path.join(output,'integration.json'),JSON.stringify({exam_id:exam.id,release_id:exam.release_id,document_id:document.id,source:process.env.REAL_OLLAMA_URL?'real NAS Ollama; AI fixture':'Ollama+AI protocol fixtures',received:2,ocr_calls:ocrCalls,ai_calls:aiCalls},null,2));
 // English teacher card: 60 questions with ABC / ABCD / A-G groups, a fixed
 // four-digit student number and the same shared exam code.
 const ecls=await request("/api/v1/classes","POST",{name:"V02 English "+Date.now()});
 await request("/api/v1/classes/"+ecls.id+"/students","POST",{students:[{number:"1932",name:"英语学生一"}]});
 const eqs=[];let qn=1;
 const push=(count,options,section)=>{for(let i=0;i<count;i++){eqs.push({number:String(qn++),type:"single_choice",options,answer:[options[0]],score_units:200,section});}};
 push(20,["A","B","C"],"第一部分 听力（1–20）");
 push(15,["A","B","C","D"],"第二部分 阅读理解（21–35）");
 push(5,["A","B","C","D","E","F","G"],"第三部分 七选五（36–40）");
 push(20,["A","B","C","D"],"第四部分 英语知识运用（41–60）");
 let eexam=await request("/api/v1/exams","POST",{title:"英语期中答题卡",subject:"英语",class_id:ecls.id,questions:eqs});
 eexam=await request("/api/v1/exams/"+eexam.id+"/publish","POST",{revision:eexam.revision});
 assert.equal(eexam.template.version,"a5-v3");assert.equal(eexam.template.student_number_digits,4);assert.equal(eexam.template.questions.length,60);
 assert(eexam.template.questions.some(q=>q.options.length===7));assert(eexam.template.questions[0].section);
 const esheets=await request("/api/v1/exams/"+eexam.id+"/sheets");
 const esvg=await request("/api/v1/exams/"+eexam.id+"/print-sheet?student_ids="+esheets.items[0].student_id);
 assert(esvg.includes("a5-v3"));assert(esvg.includes("七选五"));
 await fs.writeFile(path.join(output,"english-two-up.svg"),esvg);
 await fs.writeFile(path.join(output,"english-manifest.json"),JSON.stringify(eexam.template));
 const eout=path.join(output,"english-card.png");
 execFileSync(fixtureBinary,["-manifest",path.join(output,"english-manifest.json"),"-number","1932","-token",esheets.items[0].token,"-answers",Array.from({length:60},()=>"A").join(","),"-output",eout],{stdio:"pipe"});
 const eform=new FormData(),eid=randomBytes(16).toString("hex");
 eform.set("image",new Blob([await fs.readFile(eout)],{type:"image/png"}),"english.png");
 eform.set("client_capture_id",eid);
 eform.set("candidate_sheet_token",esheets.items[0].token);
 const eaccepted=await request("/api/v1/exams/"+eexam.id+"/captures","POST",eform,{"Idempotency-Key":eid});
 const ecommit=await until(async()=>{const c=await request("/api/v1/captures/"+eaccepted.capture_id);if(c.status==="failed")throw Error(c.message);return c.status==="committed"?c:false});
 assert.equal(ecommit.student_number,"1932",JSON.stringify(ecommit));
 assert.equal(ecommit.score_units,ecommit.max_score_units,JSON.stringify(ecommit));
 await fs.writeFile(path.join(output,"integration.json"),JSON.stringify({exam_id:exam.id,release_id:exam.release_id,document_id:document.id,english_exam_id:eexam.id,english_capture_id:ecommit.capture_id,source:process.env.REAL_OLLAMA_URL?"real NAS Ollama; AI fixture":"Ollama+AI protocol fixtures",received:2,english_received:1,ocr_calls:ocrCalls,ai_calls:aiCalls},null,2));
 console.log('PASS v0.2: encrypted live Ollama settings -> persisted OCR -> OpenAI analysis -> explicit adoption/missing answer block -> teacher key -> a5-v2 -> shared exam QR A4 two-up -> actual student-number Go OMR0001+0002 -> idempotent scores -> regrade -> restart persistence -> English a5-v3 60-question ABC/ABCD/A-G two-up -> real-pixel Go OMR 1932 full marks. AI uses an HTTP fixture, not a real paid provider.');
}finally{for(const p of children)if(p.exitCode===null){p.kill('SIGTERM');await Promise.race([new Promise(r=>p.once('exit',r)),new Promise(r=>setTimeout(r,3000))]);if(p.exitCode===null)p.kill('SIGKILL')}await new Promise(r=>fixture.close(r));execFileSync(sandboxBinary,['-drop',schema],{stdio:'pipe'});}
