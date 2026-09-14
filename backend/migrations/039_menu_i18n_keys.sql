use kol_admin;

-- Database-driven menu titles must be locale keys rather than display-language text.
update sys_menus set title = case id
  when 900 then 'menus.businessOperations'
  when 901 then 'menus.businessResources'
  when 902 then 'menus.businessTags'
  when 903 then 'menus.businessProjects'
  when 904 then 'menus.businessBriefs'
  when 905 then 'menus.businessDashboard'
  when 906 then 'menus.businessAssistant'
  when 907 then 'menus.businessGovernance'
  when 909 then 'menus.businessResourcePosts'
  when 1005 then 'menus.systemCollectionControls'
  when 1006 then 'menus.systemStandardFields'
  when 1105 then 'menus.monitorImportSync'
  else title end
where id in (900,901,902,903,904,905,906,907,909,1005,1006,1105);
