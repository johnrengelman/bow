package main

import (
	"say"
	"conf"
	"db"
	"strconv"
	"net/url"
	"net/http"
	"html/template"
	"encoding/json"
	"checker"
	_ "github.com/wader/disable_sendfile_vbox_linux"
)
func main() {
	conf.Init()
	db.Init()

	http.Handle("/resources/", http.StripPrefix("/resources/", http.FileServer(http.Dir("resources"))))

	http.HandleFunc("/managerepos/", mrepoHandler)
	http.HandleFunc("/info/", infoHandler)
	http.HandleFunc("/upgrade/", upgradeHandler)
	http.HandleFunc("/delete", deleteHandler)
	http.HandleFunc("/", welcomeHandler)

	go checker.DaemonManager()

	say.Info("Server listening at [" + conf.Env["servadd"] + "]")
	if err := http.ListenAndServe(conf.Env["servadd"], nil); err != nil {
		say.Error(err.Error() + "\nListenAndServe()\nmain()\nmain.go\nmain")
	}
}
func welcomeHandler(w http.ResponseWriter, r *http.Request){
	repos := db.GetRepos()
	irepos := make(map[string]interface{}, len(repos))
	irepos["repos"] = repos
	renderTemplate(w, "welcome", irepos)
}
func mrepoHandler(w http.ResponseWriter, r *http.Request){
	urlc := r.URL.Path[len("/managerepos/"):]
	repos := db.GetRepos()
	var repopretty map[string]string
	if urlc == "add" {
		if v, err := url.ParseQuery(r.URL.RawQuery); err != nil {
			say.Raw(err)
		} else {
			if len(v) != 0 {
				db.CreateRepo(v)
				http.Redirect(w, r, "/managerepos/", 307)
			}
		}
	}
	if urlc == "edit" {
		if v, err := url.ParseQuery(r.URL.RawQuery); err != nil {
			say.Raw(err)
		} else {
			if len(v) == 1 {
				repopretty = db.GetRepoPretty(v["reponame"][0])
				repopretty["repopass"] = "*********"
			}
			if len(v) > 1 {
				db.CreateRepo(v)
				http.Redirect(w, r, "/managerepos/", 307)
			}
		}
	}
	if urlc == "delete" {
		if v, err := url.ParseQuery(r.URL.RawQuery); err != nil {
			say.Raw(err)
		} else {
			if len(v) == 1 {
				db.DeleteRepo(v["reponame"][0])
				http.Redirect(w, r, "/managerepos/", 307)
			}
		}
	}
	irepos := make(map[string]interface{}, len(urlc)+len(repos)+len(repopretty))

	irepos["path"] = urlc
	irepos["repos"] = repos
	irepos["chosen"] = repopretty

	renderTemplate(w, "managerepos", irepos)
}
func infoHandler(w http.ResponseWriter, r *http.Request){
	irepos := make(map[string]interface{})
	irepos["reponame"] = r.URL.Path[len("/info/"):]
	repo := db.GetRepoPretty(irepos["reponame"].(string))
	irepos["header"] = irepos["reponame"].(string) + " : " + repo["repohost"]

	if v, err := url.ParseQuery(r.URL.RawQuery); err != nil {
		say.Raw(err)
	} else {
		if len(v) != 0 {
			if v["curname"] != nil {
				irepos["curname"] = v["curname"][0]

				tags := db.GetTags(irepos["reponame"].(string), irepos["curname"].(string))
				uploads := make(map[string]map[string]string)
				totaluploads := make(map[string]int)
				for _, e := range tags {
					uploads[e] = make(map[string]string)
					uploads[e] = db.GetSimplePairsFromBucket([]string{
						irepos["reponame"].(string),
						"catalog",
						irepos["curname"].(string),
						e,
						"_uploads" })
					count := 0
					for _, eu := range uploads[e] {
						if num, err := strconv.Atoi(eu); err != nil {
							say.Error(err.Error())
						} else {
							count += num
						}
					}
					totaluploads[e] = count
				}
				irepos["tags"] = totaluploads
				irepos["header"] = irepos["header"].(string) + "/" + irepos["curname"].(string)
				if v["curtag"] != nil {
					irepos["curtag"] = v["curtag"][0]
					irepos["uploads"] = uploads[irepos["curtag"].(string)]
					irepos["header"] = irepos["header"].(string) + ":" + irepos["curtag"].(string)
					var dbpath = []string{
						irepos["reponame"].(string),
						"catalog",
						irepos["curname"].(string),
						irepos["curtag"].(string),
						"history" }
					strhist := db.GetSimplePairsFromBucket(dbpath)
					objhist := make(map[string]interface{})
					lastkey := ""
					layersnum := 0
					for key, value := range  strhist {
						var ch interface{}
						_ = json.Unmarshal([]byte(value), &ch)
						objhist[key] = ch
						if lastkey < key {
							lastkey = key
						}
						layersnum++
					}
					irepos["history"] = objhist
					irepos["lastupdated"] = lastkey
					irepos["layersnum"] = layersnum
					dbpath[4] = "_totalsizehuman"
					strsizehuman := db.GetSimplePairsFromBucket(dbpath)
					dbpath[4] = "_totalsizebytes"
					strsizebytes := db.GetSimplePairsFromBucket(dbpath)
					lastkey = ""
					for key, _ := range strsizehuman {
						if lastkey < key {
							lastkey = key
						}
					}
					if strsizebytes != nil {
						irepos["imagesizebytes"] = strsizebytes
					}
					if strsizehuman != nil {
						irepos["imagesizehuman"] = strsizehuman
					}
					irepos["lastpushed"] = lastkey
					dbpath[4] = "_parent"
					irepos["parent"] = db.GetSimplePairsFromBucket(dbpath)
				}
			}
		}
	}

	irepos["catalog"] = db.GetCatalog(irepos["reponame"].(string))
	renderTemplate(w, "info", irepos)
}
func upgradeHandler(w http.ResponseWriter, r *http.Request){
	funcname := r.URL.Path[len("/upgrade/"):]
	say.Info("Starting upgrade for [ " + funcname + " ]")
	if funcname == "totalsize" {
		db.UpgradeTotalSize()
	}
	if funcname == "falsenumnames" {
		db.UpgradeFalseNumericImage()
	}
	if funcname == "oldparentnames" {
		db.UpgradeOldParentNames()
	}
	http.Redirect(w, r, "/", 307)
}
func deleteHandler(w http.ResponseWriter, r *http.Request){
	if v, err := url.ParseQuery(r.URL.RawQuery); err != nil {
		say.Raw(err)
	} else {
		if (v["reponame"] != nil) && (v["curname"] != nil) && (v["curtag"] != nil) {
			say.Info("Starting delete manifest [ " + v["reponame"][0] + "/" + v["curname"][0] + "/" + v["curtag"][0] + " ]")
			say.Raw(checker.DeleteTagFromRepo(v["reponame"][0], v["curname"][0], v["curtag"][0]))
			http.Redirect(w, r, "/info/" + v["reponame"][0] + "?curname=" + v["curname"][0], 307)
		} else {
			say.Error("Something wrong with args in deleteHandler")
		}
	}
}

func renderTemplate(w http.ResponseWriter, tmpl string, c interface{}) {
	say.Info("Rendering template [ " + tmpl + " ]")
	templates := template.Must(template.ParseGlob("./templates/*"))
	err := templates.ExecuteTemplate(w, tmpl, c)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
