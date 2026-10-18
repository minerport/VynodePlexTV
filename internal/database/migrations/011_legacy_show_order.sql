-- Convert only the exact legacy single-show template; leave mixed/custom rules intact.
CREATE TEMP TABLE legacy_show_channels AS
SELECT channel_id,json_extract(config_json,'$.rule.root.children[1].value') AS show_id
FROM channel_sources WHERE source_type='RULE'
AND json_extract(config_json,'$.recent') IS NULL
AND json_extract(config_json,'$.rule.root.operator')='AND'
AND json_array_length(config_json,'$.rule.root.children')=3
AND json_extract(config_json,'$.rule.root.children[0].field')='availability'
AND json_extract(config_json,'$.rule.root.children[0].operator')='is_true'
AND json_extract(config_json,'$.rule.root.children[1].field')='show'
AND json_extract(config_json,'$.rule.root.children[1].operator')='is'
AND json_type(config_json,'$.rule.root.children[1].value')='text'
AND json_extract(config_json,'$.rule.root.children[2].field')='media_type'
AND json_extract(config_json,'$.rule.root.children[2].operator')='is'
AND json_extract(config_json,'$.rule.root.children[2].value')='episode';
UPDATE channel_sources SET source_type='SHOW',config_json=json_object('type','SHOW','show',json_object('showId',(SELECT show_id FROM legacy_show_channels WHERE channel_id=channel_sources.channel_id),'episodeScope','ALL','seasonIds',json('[]'),'includeSpecials',json('true'))),updated_at=unixepoch()
WHERE channel_id IN(SELECT channel_id FROM legacy_show_channels);
UPDATE channels SET channel_type='SHOW',updated_at=unixepoch() WHERE id IN(SELECT channel_id FROM legacy_show_channels);
UPDATE channel_schedule_settings SET mode='SEQUENTIAL',updated_at=unixepoch() WHERE channel_id IN(SELECT channel_id FROM legacy_show_channels);
INSERT INTO jobs(id,type,channel_id,status,created_at)
SELECT lower(hex(randomblob(16))),'SCHEDULE_REPAIR',channel_id,'QUEUED',unixepoch() FROM legacy_show_channels l
WHERE NOT EXISTS(SELECT 1 FROM jobs j WHERE j.channel_id=l.channel_id AND j.type LIKE 'SCHEDULE_%' AND j.status IN('QUEUED','RUNNING'));
DROP TABLE legacy_show_channels;
