// 插件, 不是立刻就需要的功能
// 1. 上传, 粘贴图片2
// 2. 笔记信息
// 3. 历史记录
// 4. 附件
requirejs.config({
    paths: {
        // life
        'editor_drop_paste': 'js/plugins/editor_drop_paste',
        'attachment_upload': 'js/plugins/attachment_upload',

        'note_info': 'js/plugins/note_info',
        'tips': 'js/plugins/tips',
        'history': 'js/plugins/history',
    },
});

// 异步加载
setTimeout(function () {
    // 小异步
    require(["editor_drop_paste", "attachment_upload"]);

    require(['note_info']);

    // 大异步
    setTimeout(function () {
        require(['tips']);
        require(['history']);
    }, 10);
});