$(function () {
    function humanReadable(size) {
        if (size < 1024) {
            return size + " byte"
        } else if (size < 1024 * 1024) {
            return (size / 1024).toFixed(2) + " KB"
        } else if (size < 1024 * 1024 * 1024) {
            return (size / 1024 / 1024).toFixed(2) + " MB"
        } else if (size < 1024 * 1024 * 1024 * 1024) {
            return (size / 1024 / 1024 / 1024).toFixed(2) + " GB"
        }
    }

    function refreshInfo(initial) {
        if (initial || $("#nav_crawler").attr("class").indexOf("active") !== -1) {
            $.get("/admin/monitor?type=crawler", function (data, status) {
                const json = JSON.parse(data)
                if (json.code !== 0) {
                    return
                }
                let html = ""
                for (let i in json.data) {
                    let info = json.data[i]
                    if (info.dead === true) {
                        info.dead = "<span style='color: red;font-weight: bold;'>死亡</span>"
                        info.mem_total = info.mem_percent = info.cpu_percent = info.running_time = ""
                        info.crawled_count = info.failure_count = info.failure_rate = ""
                    } else {
                        info.dead = "<span style='color: limegreen; font-weight: bold'>存活</span>"
                        info.mem_percent = (info.mem_percent * 100).toFixed(2) + "%"
                        info.cpu_percent = info.cpu_percent.toFixed(2) + "%"
                        info.failure_rate = (info.failure_rate.toFixed(2) * 100) + "%"
                        info.mem_total = humanReadable(info.mem_total)
                    }
                    html += "<tr>" +
                        "<td>" + info.addr + "</td>" +
                        "<td>" + info.dead + "</td>" +
                        "<td>" + info.mem_total + "</td>" +
                        "<td>" + info.mem_percent + "</td>" +
                        "<td>" + info.cpu_percent + "</td>" +
                        "<td>" + info.running_time + "</td>" +
                        "<td>" + info.crawled_count + "</td>" +
                        "<td>" + info.failure_count + "</td>" +
                        "<td>" + info.failure_rate + "</td>" +
                        "</tr>"
                }
                $("#table_crawler tbody").html(html)
            })
        }
        if (initial || $("#nav_indexer").attr("class").indexOf("active") !== -1) {
            $.get("/admin/monitor?type=indexer", function (data, status) {
                const json = JSON.parse(data)
                if (json.code !== 0) {
                    return
                }
                let html = ""
                for (let i in json.data) {
                    let info = json.data[i]
                    if (info.dead === true) {
                        info.dead = "<span style='color: red; font-weight: bold'>死亡</span>"
                        info.mem_total = info.mem_percent = info.cpu_percent = info.running_time = ""
                        info.index_size = info.indexed_doc_count = info.token_count = ""
                    } else {
                        info.dead = "<span style='color: limegreen; font-weight: bold'>存活</span>"
                        info.cpu_percent = info.cpu_percent.toFixed(2) + "%"
                        info.mem_percent = (info.mem_percent * 100).toFixed(2) + "%"
                        info.mem_total = humanReadable(info.mem_total)
                        info.index_size = humanReadable(info.index_size)
                    }
                    html += "<tr>" +
                        "<td>" + info.addr + "</td>" +
                        "<td>" + info.dead + "</td>" +
                        "<td>" + info.mem_total + "</td>" +
                        "<td>" + info.mem_percent + "</td>" +
                        "<td>" + info.cpu_percent + "</td>" +
                        "<td>" + info.running_time + "</td>" +
                        "<td>" + info.index_size + "</td>" +
                        "<td>" + info.indexed_doc_count + "</td>" +
                        "<td>" + info.token_count + "</td>" +
                        "</tr>"
                }
                $("#table_indexer tbody").html(html)
            })
        }
    }

    refreshInfo(true)
    setInterval(refreshInfo, 5000, false)


    ////////////域名、关键词管理///////////
    $("#btn_include").click(function () {
        let domainList = $("#domain").val().trim()
        $.post("/admin/include_domain", {domain:domainList}, function (data, status){
            if (status !== "success" || data.code !== 0) {
                alert("收录失败")
                return
            }
            alert("收录成功")
            $("#domain").val("")
        })
    })
    $("#btn_blacklist").click(function () {
        let domainList = $("#domain").val().trim()
        $.post("/admin/manage_domain_blacklist", {domain:domainList, opType:"add"}, function (data, status){
            if (status !== "success" || JSON.parse(data).code !== 0) {
                alert("添加黑名单失败")
                return
            }
            alert("添加黑名单成功")
            $("#domain").val("")
        })
    })
    $("#btn_keyword").click(function () {
        let keywords = $("#illegal_keyword").val().trim()
        $.post("/admin/manage_illegal_keyword", {keyword:keywords, opType:"add"}, function (data, status){
            if (status !== "success" || JSON.parse(data).code !== 0) {
                alert("添加违法关键字失败")
                return
            }
            alert("添加违法关键字成功")
            $("#illegal_keyword").val("")
        })
    })
    $("#refresh_domain_blacklist").click(function () {
        $.get("/admin/get_domain_blacklist", function (data, status) {
            if (status !== "success") {
                alert("操作失败")
                return
            }
            const json = JSON.parse(data)
            if (json.code !== 0) {
                alert("操作失败")
                return
            }
            let html = ""
            for (let i in json.data) {
                let d = json.data[i]
                html += "<tr>" +
                    "<td>" + d + "</td>" +
                    "<td>" +
                        "<button class='btn btn-sm btn-primary del-domain' data-domain='" + d + "'>删除</button>" +
                    "</td>" +
                    "</tr>"
            }
            $("#table_domain_blacklist tbody").html(html)

            $(".del-domain").click(function (){
                const domain = $(".del-domain").attr("data-domain")
                $.post("/admin/manage_domain_blacklist", {domain:domain, opType:"del"}, function (data, status) {
                    if (status !== "success" || JSON.parse(data).code !== 0) {
                        alert("操作失败")
                        return
                    }
                    alert("操作成功")
                    $("#refresh_domain_blacklist").click()
                })
            })
        })
    })
    $("#refresh_illegal_keyword").click(function () {
        $.get("/admin/get_illegal_keyword", function (data, status) {
            if (status !== "success") {
                alert("操作失败")
                return
            }
            const json = JSON.parse(data)
            if (json.code !== 0) {
                alert("操作失败")
                return
            }
            let html = ""
            for (let i in json.data) {
                let k = json.data[i]
                html += "<tr>" +
                    "<td>" + k + "</td>" +
                    "<td>" +
                        "<button class='btn btn-sm btn-primary del-keyword' data-keyword='" + k + "'>删除</button>" +
                    "</td>" +
                    "</tr>"
            }
            $("#table_illegal_keyword tbody").html(html)

            $(".del-keyword").click(function(){
                const keyword = $(".del-keyword").attr("data-keyword")
                $.get("/admin/manage_illegal_keyword", {keyword:keyword, opType:"del"}, function (data, status) {
                    if (status !== "success" || JSON.parse(data).code !== 0) {
                        alert("操作失败")
                        return
                    }
                    alert("操作成功")
                    $("#refresh_illegal_keyword").click()
                })
            })
        })
    })

    ///////////////////爬虫管理////////////////////
    $("").click(function () {
        $.get("/admin/get_illegal_keyword", function (data, status) {

        })
    })
    $("").click(function () {
        $.get("/admin/get_illegal_keyword", function (data, status) {

        })
    })
    $("").click(function () {
        $.get("/admin/get_illegal_keyword", function (data, status) {

        })
    })
    $("").click(function () {
        $.get("/admin/get_illegal_keyword", function (data, status) {

        })
    })
    $("").click(function () {
        $.get("/admin/get_illegal_keyword", function (data, status) {

        })
    })
    $("").click(function () {
        $.get("/admin/get_illegal_keyword", function (data, status) {

        })
    })
})