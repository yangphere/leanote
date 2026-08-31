(function (window) {
	var localeMap = {
		'de-de': 'de-DE', 'en-us': 'en-US', 'es-co': 'es-CO', 'fr-fr': 'fr-FR',
		'pt-pt': 'pt-PT', 'zh-cn': 'zh-CN', 'zh-hk': 'zh-HK'
	};
	var fontFormats = 'Arial=arial,helvetica,sans-serif;Arial Black=arial black,avant garde;'
		+ 'Times New Roman=times new roman,times;Courier New=courier new,courier;'
		+ 'Tahoma=tahoma,arial,helvetica,sans-serif;Verdana=verdana,geneva;'
		+ '宋体=SimSun;新宋体=NSimSun;黑体=SimHei;微软雅黑=Microsoft YaHei';

	function resolveLocale(locale) {
		var normalized = String(locale || '').toLowerCase();
		var language = localeMap[normalized];
		if (!language) throw new Error('Unsupported TinyMCE locale: ' + locale);
		return { language: language, language_url: '/tinymce/langs/' + normalized + '.js' };
	}

	function createBaseConfig(options) {
		var locale = resolveLocale(options.locale);
		return {
			inline: true, selector: options.selector, license_key: 'gpl',
			base_url: '/tinymce', suffix: '.min', language: locale.language,
			language_url: locale.language_url, convert_urls: false, relative_urls: true,
			remove_script_host: false, menubar: false, statusbar: false,
			font_family_formats: fontFormats,
			block_formats: 'Header 1=h1;Header 2=h2;Header 3=h3;Header 4=h4;Pre=pre;Paragraph=p'
		};
	}

	function createNoteConfig(options) {
		var config = createBaseConfig(options);
		config.plugins = ['autolink', 'link', 'lists', 'searchreplace', 'table', 'leaui_image', 'leaui_mindmap', 'leanote_nav', 'leanote_code'];
		config.toolbar = 'blocks | forecolor backcolor | bold italic underline strikethrough | leaui_image leaui_mindmap | leanote_code leanote_inline_code | bullist numlist | alignleft aligncenter alignright alignjustify || outdent indent blockquote | link unlink | table | hr removeformat | subscript superscript | searchreplace | fontfamily fontsize';
		config.valid_children = '+pre[div|#text|p|span|textarea|i|b|strong]';
		config.paste_data_images = true;
		return config;
	}

	function createMemberConfig(options) {
		var config = createBaseConfig(options);
		config.plugins = ['advlist', 'autolink', 'link', 'lists', 'charmap', 'searchreplace', 'visualblocks', 'visualchars', 'table', 'directionality'];
		config.toolbar = 'blocks | fontfamily fontsize | forecolor backcolor | bold italic underline strikethrough | bullist numlist';
		return config;
	}

	window.LeanoteTinyMCE = { resolveLocale: resolveLocale, createNoteConfig: createNoteConfig, createMemberConfig: createMemberConfig };
})(window);
