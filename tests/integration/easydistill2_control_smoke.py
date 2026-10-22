"""Explicit opt-in acceptance against a running control plane and real GCS worker.

Uses synthetic data and the fixture server from easydistill2_smoke.py. Removes
only the project/datasets created by this invocation after successful checks.
On failure it retains the labeled acceptance project for diagnosis.
"""
import argparse
import json
import time
from pathlib import Path
import requests


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--base-url',required=True)
    parser.add_argument('--uid',type=int,required=True)
    parser.add_argument('--teacher-url',required=True)
    parser.add_argument('--worker',required=True)
    parser.add_argument('--report',required=True)
    args=parser.parse_args()
    base=args.base_url.rstrip('/')+'/api/v1'
    def request(method,path,**kw):
        kw.setdefault('params',{}).setdefault('uid', args.uid)
        r=requests.request(method,base+path,timeout=30,**kw)
        if r.status_code>=400: raise RuntimeError(f'{method} {path}: {r.status_code} {r.text}')
        return r.json().get('data')
    project=request('POST','/projects',json={'uid':args.uid,'name':'EasyDistill 2 验收（合成数据）','teacher_model_config':{'provider_type':'api','model_name':'fixture','endpoint':args.teacher_url,'extra_params':{'api_key':'fixture'}}})
    seed='\n'.join(json.dumps({'instruction':f'Explain the sum of two and two, example {i}','problem':'What is 2+2?'}) for i in range(2))
    dataset=request('POST','/datasets/upload',data={'uid':str(args.uid),'name':'EasyDistill 2 synthetic seeds'},files={'file':('seeds.jsonl',seed,'application/json')})
    assert request('GET',f'/datasets/{dataset["id"]}/preview?uid={args.uid}')['total']==2
    assert request('GET','/runtime')['image_pull_policy']=='never'
    recipes=request('GET','/workflows')['items']
    assert len(recipes)==15 and all('default_parameters' in x for x in recipes)
    probe=request('POST','/teachers/test',json={'uid':args.uid,'teacher_model':{'model_name':'fixture','endpoint':args.teacher_url,'api_secret_ref':'fixture'}})
    assert probe['connected'] and 'chat_completion' in probe['checks']
    assert request('GET','/projects',params={'name':'EasyDistill 2 验收'})['total']>=1
    assert request('GET','/datasets',params={'name':'EasyDistill 2 synthetic','source_type':'upload'})['total']>=1
    models=request('GET','/models/student')
    assert all('ready' in x for x in models)
    jobs=['instruct_distill','cot_distill','advanced_instruct_distill','dpo_data_build']
    report=[]
    for job in jobs:
        params={}
        if job=='advanced_instruct_distill':
            params={'pipeline':[{'stage':'generate','config':{}},{'stage':'instruct_eval','config':{}},{'stage':'quality_filter','config':{'min_scores':{'helpfulness':6},'keep_top_ratio':1.0}},{'stage':'build_sft','config':{}}]}
        payload={'uid':args.uid,'project_id':project['id'],'dataset_id':dataset['id'],'training_config':{'distillation':{'version':'2.0','job_type':job,'parameters':params,'train_student':False}},'resource_request':{'executor_nodes':[args.worker]}}
        preview=request('POST','/workflows/preview',json=payload)
        assert preview['config']['backend']['api_key']=='********'
        assert len(preview['stages'])==4
        pipeline=request('POST','/pipelines',json=payload)
        request('POST',f'/pipelines/{pipeline["id"]}/start')
        deadline=time.time()+150
        while time.time()<deadline:
            current=request('GET',f'/pipelines/{pipeline["id"]}')
            if current['status'] in ('succeeded','failed','canceled'): break
            time.sleep(2)
        assert current['status']=='succeeded',current
        stages=request('GET',f'/pipelines/{pipeline["id"]}/stages')
        if isinstance(stages,dict): stages=stages.get('items') or stages.get('stages')
        assert len(stages)==4 and all(s['status']=='succeeded' for s in stages),stages
        output=next(s for s in stages if s['stage_type']=='dataset_export')['output_manifest']
        generated=request('GET',f'/datasets/{output["dataset_id"]}?uid={args.uid}')
        assert generated['source_type']=='generated' and generated['record_count']>0
        artifacts=request('GET',f'/pipelines/{pipeline["id"]}/artifacts?uid={args.uid}')['items']
        artifact=next(a for a in artifacts if a['relative_path'].startswith('data/exported/dataset.'))
        rows=request('GET',f'/pipelines/{pipeline["id"]}/artifacts/preview',params={'uid':args.uid,'path':artifact['relative_path']})
        assert rows['total']>0
        quality=request('GET',f'/pipelines/{pipeline["id"]}/quality')
        assert quality['token_usage'] is None and quality['model_evaluation']=='not_performed'
        if job=='advanced_instruct_distill':
            m=next(x['metrics']['helpfulness'] for x in quality['scored_artifacts'] if 'helpfulness' in x['metrics'])
            assert {k:m[k] for k in ['count','min','max','mean']}=={'count':2,'min':2,'max':8,'mean':5},m
            assert m['histogram']=={'2.0':1,'8.0':1},m
        filtered=request('GET','/pipelines',params={'name':current['name'],'status':'succeeded','job_type':job})
        assert filtered['total']==1,filtered
        for path in [f'/datasets/{generated["id"]}/download',f'/pipelines/{pipeline["id"]}/artifacts/download']:
            params={'uid':args.uid}
            if 'artifacts' in path:params['path']=artifact['relative_path']
            response=requests.get(base+path,params=params,timeout=30)
            assert response.status_code==200 and response.content
        stage=next(x for x in stages if x['stage_type']=='distill_data')
        for suffix in ['/logs','/logs/download']:
            response=requests.get(base+f'/pipelines/{pipeline["id"]}/stages/{stage["id"]}'+suffix,params={'uid':args.uid},timeout=30)
            assert response.status_code==200 and len(response.content)>20
        for path in [f'/pipelines/{pipeline["id"]}',f'/pipelines/{pipeline["id"]}/quality']:
            assert requests.get(base+path,params={'uid':args.uid+100000},timeout=10).status_code==404
        assert current['summary']['output_records']==rows['total'] and generated['lineage']['pipeline']==pipeline['id']
        training=request('GET',f'/datasets/{generated["id"]}/training-config')
        assert training['stage']==('dpo' if job=='dpo_data_build' else 'sft') and not training['creates_training_task']
        if job=='instruct_distill':
            copy=request('POST',f'/pipelines/{pipeline["id"]}/clone',json={'name':'验收副本'})
            assert copy['id']!=pipeline['id'] and copy['status']=='pending'
            assert copy['training_config']['distillation']['teacher_model']['extra_params']['api_key']=='********'
            request('POST',f'/pipelines/{copy["id"]}/cancel')
            assert request('GET',f'/pipelines/{copy["id"]}')['status']=='canceled'
            response=requests.post(base+f'/pipelines/{copy["id"]}/start',params={'uid':args.uid},timeout=20)
            assert response.status_code==409,response.text
        report.append({'job_type':job,'pipeline_id':pipeline['id'],'records':rows['total'],'dataset_id':output['dataset_id'],'stages':[s['stage_type'] for s in stages],'status':'passed'})
        print(json.dumps(report[-1]),flush=True)
    summary=request('GET',f'/projects/{project["id"]}/summary?uid={args.uid}')
    assert summary['total']==5 and summary['status_counts']=={'succeeded':4,'canceled':1},summary
    Path(args.report).write_text(json.dumps({'result':'EASYDISTILL2_CONTROL_SMOKE_OK','tests':report},ensure_ascii=False,indent=2))
    request('DELETE',f'/projects/{project["id"]}?uid={args.uid}')
    request('DELETE',f'/datasets/{dataset["id"]}?uid={args.uid}')
    for item in report: request('DELETE',f'/datasets/{item["dataset_id"]}?uid={args.uid}')
    print('EASYDISTILL2_CONTROL_SMOKE_OK',flush=True)


if __name__=='__main__': main()
