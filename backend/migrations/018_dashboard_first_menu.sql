use kol_admin;

update sys_menus set
  parent_id = 900,
  title = '看板',
  path = '/business/dashboard',
  name = 'BusinessDashboard',
  component = 'business/dashboard/index',
  `rank` = 1,
  icon = 'ri:bar-chart-box-line',
  show_link = 1
where id = 905 or path = '/business/dashboard';

update sys_menus set
  parent_id = 900,
  title = 'Campaign 执行中心',
  path = '/business/projects',
  name = 'BusinessProjects',
  component = 'business/projects/index',
  `rank` = 4,
  icon = 'ri:flow-chart',
  show_link = 1
where id = 903 or path = '/business/projects';

update sys_menus set
  parent_id = 900,
  title = 'AI模型配置',
  path = '/business/ai-model',
  name = 'BusinessAIModel',
  component = 'business/ai-model/index',
  `rank` = 9,
  icon = 'ri:sparkling-2-line',
  show_link = 1
where id = 908 or path = '/business/ai-model';

update sys_menus
set `rank` = case path
  when '/business/resources' then 2
  when '/business/tags' then 3
  when '/business/projects' then 4
  when '/business/briefs' then 5
  when '/business/resource-posts' then 6
  when '/business/assistant' then 7
  when '/business/governance' then 8
  when '/business/ai-model' then 9
  else `rank`
end
where parent_id = 900;
