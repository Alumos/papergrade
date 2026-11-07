// Against an explicitly disposable development/test installation only.
// Creates a class, student, published exam. Never run against real teacher data.
import assert from 'node:assert/strict';
const base=process.env.SMOKE_BASE_URL ?? 'http://127.0.0.1:8080';
const origin=process.env.SMOKE_ORIGIN ?? base;
const username=process.env.ADMIN_USERNAME;
const password=process.env.ADMIN_PASSWORD;
if(!username||!password) throw new Error('Set ADMIN_USERNAME/ADMIN_PASSWORD for a disposable test installation');
let cookie='';
async function req(path,method='GET',body){const headers={Origin:origin};if(cookie)headers.Cookie=cookie;if(body)headers['Content-Type']='application/json';const r=await fetch(base+path,{method,headers,body:body?JSON.stringify(body):undefined});const c=r.headers.get('set-cookie');if(c)cookie=c.split(';')[0];const text=await r.text();let data;try{data=JSON.parse(text)}catch{data=text};if(!r.ok)throw new Error(method+' '+path+': '+r.status+' '+JSON.stringify(data));return data;}
assert.equal((await req('/readyz')).status,'ready');
const login=await req('/api/v1/auth/login','POST',{username,password});
assert.equal(login.user.username,username);
const cls=await req('/api/v1/classes','POST',{name:'Smoke '+Date.now()});
await req('/api/v1/classes/'+cls.id+'/students','POST',{students:[{number:'0001',name:'测试学生'}]});
const roster=await req('/api/v1/classes/'+cls.id+'/students');assert.equal(roster.items[0].number,'0001');
const exam=await req('/api/v1/exams','POST',{title:'Smoke objective exam',subject:'test',class_id:cls.id,questions:[{number:'1',type:'single_choice',options:['A','B','C','D'],answer:['B'],score_units:200}]});
const published=await req('/api/v1/exams/'+exam.id+'/publish','POST',{revision:exam.revision});
assert.equal(published.status,'published');
const manifest=await req('/api/v1/exams/'+exam.id+'/template');assert.equal(manifest.version,'a4-v1');
const sheets=await req('/api/v1/exams/'+exam.id+'/sheets');assert.equal(sheets.items.length,1);
const card=await req('/api/v1/exams/'+exam.id+'/cards?student_id='+roster.items[0].id);assert(card.includes('<svg'));
const results=await req('/api/v1/exams/'+exam.id+'/results');assert.equal(results.summary.expected,1);assert.equal(results.summary.received,0);
await req('/api/v1/auth/logout','POST',{});
console.log('HTTP smoke passed: auth / roster / publish / manifest / personal SVG / empty results / logout. NOT real cloud or camera acceptance.');
