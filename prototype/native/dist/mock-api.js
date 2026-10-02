(function(){
  const now=new Date().toISOString();
  const user={id:1,username:'demo-user',email:'demo@xingqiao.test',role:'user',balance:30,concurrency:3,status:'active',allowed_groups:null,balance_notify_enabled:false,balance_notify_threshold:null,balance_notify_extra_emails:[],created_at:now,updated_at:now};
  const groups=[
    {id:1,name:'GPT-Pro',platform:'openai',status:'active',rate:1,rate_multiplier:1,tool_ids:['codex'],current_operational:true},
    {id:2,name:'Claude-常规',platform:'anthropic',status:'active',rate:1,rate_multiplier:1,tool_ids:['claude'],current_operational:false},
    {id:3,name:'Grok-特惠',platform:'xai',status:'active',rate:.8,rate_multiplier:.8,tool_ids:['grok'],current_operational:false},
    {id:4,name:'DeepSeek',platform:'deepseek',status:'disabled',rate:1,rate_multiplier:1,tool_ids:['deepseek'],current_operational:false}
  ];
  const keys=[1,2,3,4].map((id,i)=>({id,user_id:1,key:'sk-demo-'+id,name:['Codex主线路','Claude备用','Grok特惠','DeepSeek'][i],group_id:id,status:id===4?'inactive':'active',group:groups[i],ip_whitelist:[],ip_blacklist:[],last_used_at:null,last_used_ip:null,quota:0,quota_used:0,expires_at:null,created_at:now,updated_at:now,current_concurrency:0,rate_limit_5h:0,rate_limit_1d:0,rate_limit_7d:0,usage_5h:0,usage_1d:0,usage_7d:0}));
  const ok=(data)=>({code:0,data});
  const json=(data,status=200)=>new Response(JSON.stringify(data),{status,headers:{'Content-Type':'application/json'}});
  const original=window.fetch;
  window.fetch=async function(input,init){
    const url=typeof input==='string'?input:input.url; const path=new URL(url,location.origin).pathname;
    if(!path.startsWith('/api/v1')) return original.apply(this,arguments);
    if(path==='/api/v1/auth/me') return json(ok({user}));
    if(path==='/api/v1/settings/public') return json(ok({site_name:'星桥测试服',site_logo:'/xingqiao-brand-logo.png',site_subtitle:'测试站用户端',api_base_url:location.origin,contact_info:'QQ群：1080152144',doc_url:'',payment_enabled:true,passkey_enabled:false,totp_enabled:false,allow_user_view_error_requests:true,version:'prototype'}));
    if(path==='/api/v1/groups/available') return json(ok(groups));
    if(path==='/api/v1/groups/rates') return json(ok({1:1,2:1,3:.8,4:1}));
    if(path==='/api/v1/keys' && (!init||!init.method||init.method==='GET')) return json(ok({items:keys,total:keys.length,page:1,page_size:100}));
    if(path==='/api/v1/monitor-v4') return json(ok({contract_version:'2',window:new URL(url,location.origin).searchParams.get('window')||'1h',refresh_interval_seconds:0,generated_at:now,groups:groups.map((g,i)=>({id:g.id,name:g.name,platform:g.platform,tool_ids:g.tool_ids,rate_multiplier:g.rate_multiplier,success_rate:i===0?97:i===1?75:i===2?68:null,request_count:i===3?0:100,success_count:i===0?97:i===1?75:i===2?68:0,real_request_count:i===3?0:100,real_success_count:i===0?97:i===1?75:i===2?68:0,probe_fallback_bucket_count:0,probe_fallback_request_count:0,ttft_p50_ms:i===0?420:null,latency_p50_ms:i===0?900:null,ttft_p95_ms:null,ttft_sample_count:0,latency_p95_ms:null,latency_sample_count:0,cache_hit_rate:null,cache_read_tokens:0,cache_creation_tokens:0,cache_hit_denominator:0,source_updated_at:now,current_operational:g.current_operational}))}));
    if(path==='/api/v1/usage' || path==='/api/v1/usage/stats' || path.startsWith('/api/v1/usage/dashboard')) return json(ok({items:[],total:0,page:1,page_size:20,total_requests:0,total_input_tokens:0,total_output_tokens:0,total_cache_tokens:0,total_cache_read_tokens:0,total_cache_creation_tokens:0,total_tokens:0,total_cost:0,total_actual_cost:0,average_duration_ms:0,trend:[],models:[],groups:[],endpoints:[]}));
    if(path==='/api/v1/payment/checkout-info') return json(ok({methods:{},global_min:1,global_max:50,plans:[],balance_disabled:false,balance_recharge_multiplier:1,subscription_usd_to_cny_rate:7,recharge_fee_rate:0,help_text:'演示环境不连接真实支付。',help_image_url:'',stripe_publishable_key:''}));
    if(path==='/api/v1/payment/orders/my') return json(ok({items:[],total:0,page:1,page_size:20}));
    if(path==='/api/v1/redeem/history') return json(ok([]));
    if(path==='/api/v1/user/profile') return json(ok(user));
    if(path==='/api/v1/user/passkeys') return json(ok([]));
    if(path==='/api/v1/user/totp/status') return json(ok({enabled:false}));
    return json(ok({}));
  };
  localStorage.setItem('ux_prototype_auth_token','demo-token');localStorage.setItem('ux_prototype_auth_user',JSON.stringify(user));
})();
