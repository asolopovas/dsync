CREATE TABLE `wp_posts` (`ID` bigint, `guid` varchar(255), `post_content` longtext);
CREATE TABLE `wp_options` (`option_id` bigint, `option_name` varchar(191), `option_value` longtext);
INSERT INTO `wp_posts` (`ID`,`guid`,`post_content`) VALUES
(1,'https://example.com/?p=1','Visit https://example.com and https:\/\/example.com'),
(2,'https://example.com/?p=2','JSON {"url":"https:\/\/example.com\/page"}');
INSERT INTO `wp_options` (`option_id`,`option_name`,`option_value`) VALUES
(1,'home','s:19:"https://example.com";'),
(2,'nested','a:1:{s:3:"url";s:19:"https://example.com";}'),
(3,'plugin','a:1:{s:6:"nested";s:43:"a:1:{s:3:"url";s:19:"https://example.com";}";}');
