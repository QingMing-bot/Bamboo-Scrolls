// 全局状态与工具
const AppState = {
  theme: 'dark',
  machines: [],
  currentJob: null,
  jobOff: null,
  histTimer: null
};

function $(sel, root=document){ return root.querySelector(sel); }
function $all(sel, root=document){ return Array.from(root.querySelectorAll(sel)); }
function setStatus(msg){ $('#statusText').textContent = msg; }
function toggleTheme(){ AppState.theme = AppState.theme==='dark'?'light':'dark'; document.body.classList.toggle('light', AppState.theme==='light'); setStatus('Theme: '+AppState.theme); }
function showToast(msg,type='info',delay=2600){
  let c = $('#toast_container'); if(!c){ console.warn('toast container missing'); return }
  const div=document.createElement('div'); div.className='toast'+(type==='success'?' success':type==='error'?' error':''); div.textContent=msg;
  c.appendChild(div);
  setTimeout(()=>{div.classList.add('hide'); setTimeout(()=>div.remove(),380);}, delay);
}

async function invoke(name, ...args){
  if(window.go && window.go.wailsapi && window.go.wailsapi.Backend && window.go.wailsapi.Backend[name]){
    return window.go.wailsapi.Backend[name](...args);
  }
  throw new Error('Binding not ready: '+name);
}

/* 路由 (三个页面): assets / control / history */
let currentPage = 'control';
function switchPage(id){
  if(id===currentPage) return;
  $all('#pages > .page').forEach(p=>p.classList.remove('active'));
  const target = $('#page-'+id);
  if(target){ target.classList.add('active'); currentPage=id; }
  $all('#navTabs .seg').forEach(seg=>seg.classList.toggle('active', seg.dataset.page===id));
  moveNavSlider();
}

function moveNavSlider(){
  const bar = $('#navTabs'); if(!bar) return;
  const slider = $('#navSlider'); if(!slider) return;
  const segs = $all('.seg', bar).filter(s=>s.id!=='navSlider');
  const active = segs.find(s=>s.classList.contains('active'));
  if(!active){ return }
  const rect = active.getBoundingClientRect();
  const barRect = bar.getBoundingClientRect();
  const w = rect.width;
  slider.style.width = w+'px';
  const offset = rect.left - barRect.left;
  slider.style.transform = 'translateX('+offset+'px)';
  bar.classList.remove('init');
}

/* Machines Page */
async function loadMachines(){
  let list = await invoke('ListMachines');
  if(!Array.isArray(list)) { list = []; }
  AppState.machines = list;
  renderMachineTable(list);
  $('#machineCount').textContent = list.length;
  setStatus('机器: '+list.length);
}
function renderMachineTable(list){
  const tbody = $('#machineTable tbody');
  tbody.innerHTML = '';
  list.forEach((m,i)=>{
    const tr = document.createElement('tr');
  tr.innerHTML = `<td>${i+1}</td><td><input type="checkbox" data-id="${m.id}"></td><td>${m.ipmi_ip}</td><td>${m.ssh_ip||''}</td><td>${m.zbx_id||''}</td><td>${m.remark||''}</td>`;
    tr.addEventListener('click', e => { if(e.target.tagName==='INPUT') return; fillMachineForm(m); });
    tbody.appendChild(tr);
  });
}
function fillMachineForm(m){
  $('#f_ipmi').value = m.ipmi_ip;
  $('#f_ssh').value = m.ssh_ip||'';
  $('#f_zbx').value = m.zbx_id||'';
  $('#f_remark').value = m.remark||'';
}
async function saveMachine(){
  const m = { id:0, ipmi_ip:$('#f_ipmi').value.trim(), ssh_ip:$('#f_ssh').value.trim(), ssh_user:'root', zbx_id:$('#f_zbx').value.trim(), remark:$('#f_remark').value.trim() };
  if(!m.ipmi_ip){ alert('IPMI 必填'); return }
  // 区分创建或更新: 先检查本地缓存是否存在
  const existed = Array.isArray(AppState.machines) && AppState.machines.some(x=>x.ipmi_ip===m.ipmi_ip);
  try { console.debug('Saving machine', m); await invoke('UpsertMachine', m); await loadMachines(); setStatus('保存成功'); showToast((existed?'更新':'创建')+'成功','success'); }
  catch(e){ showToast('创建失败: '+e,'error'); setStatus('保存失败'); }
}
async function deleteMachine(){
  const ip = $('#f_ipmi').value.trim(); if(!ip){ alert('无 IPMI'); return }
  if(!confirm('删除 '+ip+'?')) return;
  try { await invoke('DeleteMachine', ip); await loadMachines(); setStatus('删除完成'); showToast('删除成功','success'); }
  catch(e){ showToast('删除失败: '+e,'error'); setStatus('删除失败'); }
}
function getSelectedIDs(){ return $all('#machineTable tbody input[type=checkbox]:checked').map(c=>Number(c.dataset.id)); }
function selectAll(chk){ $all('#machineTable tbody input[type=checkbox]').forEach(c=>c.checked = chk.checked); }
function invertSelect(){ $all('#machineTable tbody input[type=checkbox]').forEach(c=>c.checked = !c.checked); }
function clearSelect(){ $all('#machineTable tbody input[type=checkbox]').forEach(c=>c.checked = false); }
function filterMachines(){
  const q = $('#machineSearch').value.trim().toLowerCase();
  if(!q){ renderMachineTable(AppState.machines); return; }
  const f = AppState.machines.filter(m=>[m.ipmi_ip,m.ssh_ip,m.zbx_id,m.remark].some(v=> (v||'').toLowerCase().includes(q)));
  renderMachineTable(f);
}

async function exportMachines(fmt){
  const redact = $('#export_redact').checked;
  try {
    const data = await invoke('ExportMachines', fmt, !!redact);
    const blob = new Blob([data], { type: fmt==='csv'?'text/csv':'application/json' });
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob); a.download = 'machines.'+fmt; a.click(); URL.revokeObjectURL(a.href);
    setStatus('导出'+fmt+(redact?'(脱敏)':'')+'完成');
  } catch(e){ alert('导出失败: '+e); }
}
async function importMachines(){
  const file = $('#import_file').files[0]; if(!file){ alert('未选文件'); return; }
  const text = await file.text(); const fmt = $('#import_fmt').value;
  try { const n = await invoke('ImportMachines', text, fmt); setStatus('导入 '+n+' 条'); await loadMachines(); }
  catch(e){ alert('导入失败: '+e); }
}

/* 新 批量控制 页面逻辑 */
let CtrlState = { machines:[], missing:[], selected:new Set() };
// Live 日志缓存
const LiveLog = { lines: [], max: 5000 };
function liveAppend(line){
  const pre = $('#live_log_pre');
  if(!pre) return;
  if(LiveLog.lines.push(line) > LiveLog.max){ LiveLog.lines.shift(); }
  pre.textContent = LiveLog.lines.join('\n');
  if($('#live_wrap') && $('#live_wrap').checked){ pre.style.whiteSpace='pre-wrap'; } else { pre.style.whiteSpace='pre'; }
  if($('#live_autoscroll') && $('#live_autoscroll').checked){ pre.scrollTop = pre.scrollHeight; }
  const stats=$('#live_stats'); if(stats){ stats.textContent='行数: '+LiveLog.lines.length; }
}
function liveClear(){ LiveLog.lines=[]; const pre=$('#live_log_pre'); if(pre){ pre.textContent=''; } }
function liveCopy(){ if(!LiveLog.lines.length){ showToast('无内容','info'); return; } navigator.clipboard.writeText(LiveLog.lines.join('\n')).then(()=>showToast('已复制','success'), e=>showToast('复制失败:'+e,'error')); }
function liveSave(){ if(!LiveLog.lines.length){ showToast('无内容','info'); return; } const blob=new Blob([LiveLog.lines.join('\n')],{type:'text/plain'}); const a=document.createElement('a'); a.href=URL.createObjectURL(blob); a.download='exec_live_log_'+Date.now()+'.log'; a.click(); URL.revokeObjectURL(a.href); }

async function ctrlLookup(){
  const lines = $('#ctrl_ipmis').value.split(/\n|,|;|\s+/).map(s=>s.trim()).filter(Boolean);
  if(lines.length===0){ alert('请输入 IPMI'); return }
  // 去重
  const uniqueInput = lines.filter((ip,i)=>lines.indexOf(ip)===i);
  if(uniqueInput.length !== lines.length){
    showToast('输入去重: '+lines.length+' -> '+uniqueInput.length,'info');
  }
  let ms=[]; try { ms = await invoke('MachinesLookup', uniqueInput); } catch(e){ showToast('查询失败: '+e,'error'); return }
  const foundMap=new Map(ms.map(m=>[m.ipmi_ip,m]));
  CtrlState.machines = uniqueInput.map(ip=>foundMap.get(ip)).filter(Boolean);
  CtrlState.missing = uniqueInput.filter(ip=>!foundMap.has(ip));
  CtrlState.selected = new Set(CtrlState.machines.map(m=>m.id)); // 默认全选已存在
  renderCtrlMatches();
  if(CtrlState.missing.length){
    $('#ctrl_missing').style.display='block'; $('#ctrl_missing').textContent='未登记: '+CtrlState.missing.join(', ');
    showToast('未找到主机: '+CtrlState.missing.length+' 个','error');
  } else { $('#ctrl_missing').style.display='none'; }
  if(CtrlState.machines.length>0){
    showToast('解析成功: '+CtrlState.machines.length+' 台','success');
  } else if(!CtrlState.missing.length){
    showToast('没有匹配到任何已登记主机','error');
  }
  setStatus('匹配: '+CtrlState.machines.length+' / 缺失 '+CtrlState.missing.length);
}
function renderCtrlMatches(){
  const tb = $('#ctrl_match_table tbody'); tb.innerHTML='';
  if(CtrlState.machines.length===0){ $('#ctrl_match_placeholder').style.display='flex'; return }
  $('#ctrl_match_placeholder').style.display='none';
  CtrlState.machines.forEach(m=>{
    const tr=document.createElement('tr');
    const checked = CtrlState.selected.has(m.id)?'checked':'';
  tr.innerHTML=`<td><input type="checkbox" data-id="${m.id}" ${checked}></td><td>${m.ipmi_ip}</td><td>${m.ssh_ip||''}</td><td>${m.zbx_id||''}</td>`; // 列改为 IPMI / SSH / ZBXid
    tr.querySelector('input').addEventListener('change',e=>{ const id=Number(e.target.dataset.id); if(e.target.checked) CtrlState.selected.add(id); else CtrlState.selected.delete(id); updateCtrlSelected(); });
    tb.appendChild(tr);
  });
  updateCtrlSelected();
}
function updateCtrlSelected(){ $('#ctrl_selected_count').textContent=CtrlState.selected.size; }
function ctrlSelectAll(){ CtrlState.selected=new Set(CtrlState.machines.map(m=>m.id)); renderCtrlMatches(); }
function ctrlSelectInv(){ const ns=new Set(); CtrlState.machines.forEach(m=>{ if(!CtrlState.selected.has(m.id)) ns.add(m.id); }); CtrlState.selected=ns; renderCtrlMatches(); }
function ctrlSelectNone(){ CtrlState.selected.clear(); renderCtrlMatches(); }

async function ctrlExecute(streamJob){
  const cmd=$('#ctrl_cmd').value.trim(); if(!cmd){ alert('命令为空'); return }
  const ids=[...CtrlState.selected]; if(ids.length===0){ alert('无选择'); return }
  const parallel = parseInt($('#ctrl_parallel').value)||0; const timeout=parseInt($('#ctrl_timeout').value)||30;
  const authMode = ($all('input[name=auth_mode]').find(r=>r.checked)||{value:'key'}).value;
  const password = authMode==='password'? $('#ctrl_password').value : '';
  if(authMode==='key'){
    try { const has = await invoke('HasGlobalSSHKey'); if(!has){ if(!confirm('尚未上传全局私钥，继续可能失败。仍要执行?')) return; } }
    catch(e){ console.warn('check key failed', e); }
  }
  function fmtErr(e){ if(!e) return ''; if(typeof e==='string') return e; if(e.message) return e.message; try { return JSON.stringify(e); } catch{ return String(e); } }
  function fmtLine(data){
    const ip = data.ipmi_ip || data.ipmiIP || data.ipmi || data.SSHIP || data.ssh_ip || '?';
    const out = data.stdout || data.Stdout || '';
    const err = data.error || data.Err || data.err;
    const errStr = err? (' ERR '+fmtErr(err)) : '';
    const code = (data.exit_code!=null?data.exit_code:data.ExitCode!=null?data.ExitCode:undefined);
    const usedGlobal = data.used_global_key || data.UsedGlobalKey ? ' [G]' : '';
    return ip+': '+out+(code!==undefined && out? (' (code '+code+')'):'')+errStr+usedGlobal;
  }
  const outBox=$('#ctrl_log'); outBox.textContent='执行中...';
  if(streamJob==='stream'){ // 临时流
    if(runtime && runtime.EventsOn){
      outBox.textContent=''; const pre=document.createElement('pre'); pre.className='log'; outBox.appendChild(pre);
      // ANSI 解析与着色
      const ansiColorMap = {
        30:'#666',31:'#e66',32:'#4caf50',33:'#c8b500',34:'#4aa3ff',35:'#c678dd',36:'#26c6da',37:'#eee',
        90:'#999',91:'#ff6b6b',92:'#7ad97a',93:'#ffe066',94:'#6ab0ff',95:'#ef8fff',96:'#5ed7ff',97:'#fff'
      };
      function esc(s){ return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;'); }
      function stripAnsi(s){ return s.replace(/\x1b\[[0-9;]*[A-Za-z]/g,''); }
      function ansiToHtml(s){
        let out='';
        let cur={color:null,bold:false,underline:false};
        function spanStyle(st){
          let css='';
            if(st.color) css+='color:'+st.color+';';
            if(st.bold) css+='font-weight:bold;';
            if(st.underline) css+='text-decoration:underline;';
          return css?('<span style="'+css+'">') : '';
        }
        let openSpan='';
        const parts = s.split(/(\x1b\[[0-9;]*[A-Za-z])/g);
        for(const part of parts){
          if(!part) continue;
          const m = part.match(/^\x1b\[([0-9;]*)([A-Za-z])$/);
          if(m){
            const params = m[1]; const codeType = m[2]; if(codeType==='m'){
              if(params===''){ // same as reset
                if(openSpan){ out+='</span>'; openSpan=''; }
                cur={color:null,bold:false,underline:false};
              } else {
                const codes=params.split(';').filter(Boolean).map(x=>parseInt(x,10));
                if(!codes.length) codes=[0];
                for(const c of codes){
                  if(c===0){ if(openSpan){ out+='</span>'; openSpan=''; } cur={color:null,bold:false,underline:false}; }
                  else if(c===1){ cur.bold=true; }
                  else if(c===4){ cur.underline=true; }
                  else if(c===22){ cur.bold=false; }
                  else if(c===24){ cur.underline=false; }
                  else if((c>=30&&c<=37)||(c>=90&&c<=97)){ cur.color=ansiColorMap[c]||null; }
                  else if(c===39){ cur.color=null; }
                }
                if(openSpan){ out+='</span>'; openSpan=''; }
                const style=spanStyle(cur); if(style){ out+=style; openSpan='</span>'; }
              }
            }
            continue;
          }
          out+=esc(part);
        }
        if(openSpan) out+=openSpan;
        return out;
      }
      const AnsiState={pending:''};
      function processChunk(raw){
        let s=AnsiState.pending+raw; AnsiState.pending='';
        // 处理尾部未完成序列 (如 ESC[31;)
        const incomplete = s.match(/(\x1b\[[0-9;]*)$/); if(incomplete && !/(\x1b\[[0-9;]*[A-Za-z])$/.test(s)){ AnsiState.pending=incomplete[1]; s=s.slice(0,-incomplete[1].length); }
        return ansiToHtml(s);
      }
      function appendColored(prefix, raw){
        const html = '<span class="h-prefix">'+esc(prefix)+'</span>'+processChunk(raw).replace(/\n/g,'<br>');
        pre.innerHTML += html + '<br>';
        pre.scrollTop=pre.scrollHeight;
        liveAppend(prefix+stripAnsi(raw).replace(/\n/g,''));
      }
      function appendFinalLine(l){ pre.innerHTML+=esc(l)+'<br>'; pre.scrollTop=pre.scrollHeight; liveAppend(stripAnsi(l)); }
    // 监听 chunk
      const offChunk = runtime.EventsOn('exec_chunk', data=>{ if(data.job_id) return; const ip=data.ipmi_ip||'?'; const prefix=ip+': '; const raw=(data.is_err?'\x1b[31m[ERR]\x1b[0m ':'')+data.chunk; appendColored(prefix, raw); });
      const off=runtime.EventsOn('exec_result', data=>{ if(data.job_id) return; const p=data.progress!==undefined?(' ['+Math.round(data.progress*100)+'%]'):''; const line=fmtLine(data)+p; appendFinalLine(line); });
  try { await invoke('ExecuteStreamEvents', cmd, ids, timeout, parallel, authMode, password, true); setStatus('执行完成'); }
      catch(e){ append('失败:'+e); setStatus('失败'); }
    finally { off(); offChunk(); loadHistory(); }
      return;
    }
  }
  if(streamJob==='job'){
    if(AppState.currentJob){ alert('已有任务'); return }
    if(!(runtime && runtime.EventsOn)){ alert('事件不可用'); return }
  outBox.textContent=''; const pre=document.createElement('pre'); pre.className='log'; outBox.appendChild(pre); function append(l){ pre.textContent+=l+'\n'; pre.scrollTop=pre.scrollHeight; liveAppend(l); }
    const off=runtime.EventsOn('exec_result', data=>{ if(data.job_id && data.job_id!==AppState.currentJob) return; if(!data.job_id && AppState.currentJob) return; const p=data.progress!==undefined?(' ['+Math.round(data.progress*100)+'%]'):''; const line=fmtLine(data)+p; append(line); });
    const offDone=runtime.EventsOn('exec_job_done', data=>{ if(data.job_id===AppState.currentJob){ finishCtrlJob(); setStatus('任务完成'); offDone(); }});
  try { const jobID=await invoke('StartJob','',cmd,ids,timeout,parallel,authMode,password,false); AppState.currentJob=jobID; AppState.jobOff=()=>{off();offDone();}; $('#ctrl_jobid').textContent=jobID; setStatus('任务运行:'+jobID); toggleCtrlJobButtons(true); lockPasswordField(true); }
    catch(e){ off(); offDone(); append('启动失败:'+e); setStatus('任务失败'); }
    return;
  }
  // 普通聚合
  try { const res=await invoke('ExecuteStream', cmd, ids, timeout, parallel, authMode, password); const lines=res.map(r=>fmtLine(r)); outBox.textContent=lines.join('\n'); lines.forEach(l=>liveAppend(l)); setStatus('执行完成'); }
  catch(e){ outBox.textContent='执行失败:'+e; setStatus('失败'); }
  finally { loadHistory(); }
}
function finishCtrlJob(){ if(AppState.jobOff){ AppState.jobOff(); AppState.jobOff=null; } AppState.currentJob=null; $('#ctrl_jobid').textContent=''; toggleCtrlJobButtons(false); lockPasswordField(false); }
function toggleCtrlJobButtons(r){ $('#btn_ctrl_job').disabled=r; $('#btn_ctrl_cancel').disabled=!r; }
async function ctrlCancelJob(){ if(!AppState.currentJob) return; const ok=await invoke('CancelJob', AppState.currentJob); setStatus('取消'+(ok?'成功':'失效')); setTimeout(()=>finishCtrlJob(),300); }

/* History Page */
async function loadHistory(){
  const limit = parseInt($('#hist_limit').value)||20;
  const ipf = $('#hist_ipmi').value.trim();
  const cmdf = $('#hist_cmd').value.trim();
  let hs;
  try { hs = (ipf||cmdf) ? await invoke('RecentHistoryFiltered', limit, ipf, cmdf) : await invoke('RecentHistory', limit); }
  catch(e){ $('#history_list').innerHTML = '<pre class="log">加载失败 '+e+'</pre>'; setStatus('历史加载失败'); return; }
  if(!Array.isArray(hs)) hs=[];
  $('#history_list').innerHTML = '<pre class="log">'+hs.map(h=>h.ipmi_ip+': '+h.command+' => '+(h.exit_code||h.exitCode)).join('\n')+'</pre>';
  setStatus('历史:'+hs.length);
}
function toggleHistAuto(){ if($('#hist_auto').checked){ AppState.histTimer=setInterval(loadHistory,5000); } else { clearInterval(AppState.histTimer); } }

/* Init */
function init(){
  currentPage='control'; $('#page-control').classList.add('active');
  $all('#navTabs .seg').forEach(s=> s.addEventListener('click', ()=>{ if(s.dataset.page) switchPage(s.dataset.page); }));
  // 初次定位 slider
  const bar=$('#navTabs'); if(bar){ bar.classList.add('init'); setTimeout(moveNavSlider,0); window.addEventListener('resize', ()=>{ moveNavSlider(); }); }
  loadMachines(); loadHistory();
  // 资产管理按钮事件
  const bs=$('#btn_save'); if(bs) bs.addEventListener('click', saveMachine);
  const bd=$('#btn_delete'); if(bd) bd.addEventListener('click', deleteMachine);
  const ms=$('#machineSearch');
  if(ms){ ms.addEventListener('keyup', e=>{ if(e.key==='Enter'){ filterMachines(); } }); }
  const msb=$('#btn_machine_search'); if(msb){ msb.addEventListener('click', filterMachines); }
  $('#hist_refresh').addEventListener('click', loadHistory); $('#hist_auto').addEventListener('change', toggleHistAuto);
  // 控制页事件
  $('#btn_lookup').addEventListener('click', ctrlLookup);
  $('#btn_goto_assets').addEventListener('click', ()=>{ switchPage('assets'); });
  $('#btn_clear_ipmi').addEventListener('click', ()=>{ $('#ctrl_ipmis').value=''; CtrlState={machines:[],missing:[],selected:new Set()}; renderCtrlMatches(); $('#ctrl_missing').style.display='none'; });
  const uploadKeyBtn = document.getElementById('btn_upload_key');
  if(uploadKeyBtn){
    uploadKeyBtn.addEventListener('click', ()=>{
      // 简单文件选择对话框
  const inp=document.createElement('input'); inp.type='file'; inp.accept='.rsa,.pem,.key,.txt';
      inp.onchange=async()=>{
        if(!inp.files[0]) return; const text=await inp.files[0].text();
        try{ await invoke('SetGlobalSSHKey', text); alert('SSH Key 已上传(全局)'); }
        catch(e){ alert('上传失败:'+e); }
      };
      inp.click();
    });
  }
  $('#btn_match_all').addEventListener('click', ctrlSelectAll); $('#btn_match_inv').addEventListener('click', ctrlSelectInv); $('#btn_match_none').addEventListener('click', ctrlSelectNone);
  $('#btn_ctrl_exec').addEventListener('click', ()=>ctrlExecute($('#ctrl_stream').checked?'stream':'normal'));
  $('#btn_ctrl_job').addEventListener('click', ()=>ctrlExecute('job')); $('#btn_ctrl_cancel').addEventListener('click', ctrlCancelJob);
  // 打开独立日志页按钮
  const openLiveBtn=$('#btn_open_live_log'); if(openLiveBtn){ openLiveBtn.addEventListener('click', ()=>{ switchPage('live'); }); }
  // Live 页面控件
  const liveWrap=$('#live_wrap'); if(liveWrap){ liveWrap.addEventListener('change', ()=>{ const pre=$('#live_log_pre'); if(pre){ pre.style.whiteSpace=liveWrap.checked?'pre-wrap':'pre'; } }); }
  const liveClearBtn=$('#live_clear'); if(liveClearBtn){ liveClearBtn.addEventListener('click', ()=>{ liveClear(); showToast('已清空','info'); }); }
  const liveCopyBtn=$('#live_copy'); if(liveCopyBtn){ liveCopyBtn.addEventListener('click', liveCopy); }
  const liveSaveBtn=$('#live_save'); if(liveSaveBtn){ liveSaveBtn.addEventListener('click', liveSave); }
  // 认证模式切换
  $all('input[name=auth_mode]').forEach(r=> r.addEventListener('change', ()=>{
    const m = ($all('input[name=auth_mode]').find(x=>x.checked)||{value:'key'}).value;
    const pf = $('#password_field');
    if(m==='password'){ pf.style.display='block'; } else { pf.style.display='none'; $('#ctrl_password').value=''; }
  }));
  function lockPasswordField(lock){ const inp=$('#ctrl_password'); if(!inp) return; inp.disabled=lock; inp.style.opacity=lock?'.5':'1'; }
  window.lockPasswordField = lockPasswordField;
  // 双击状态条切换主题
  $('#statusbar').addEventListener('dblclick', toggleTheme);
}

window.addEventListener('DOMContentLoaded', init);
// 全局错误捕获
window.addEventListener('error', e=>{ showToast('JS错误: '+e.message,'error',4000); });
window.addEventListener('unhandledrejection', e=>{ showToast('未处理 Promise: '+(e.reason&&e.reason.message?e.reason.message:e.reason),'error',4000); });
