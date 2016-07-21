package checker

import (
  "db"
  "say"
  "time"
  "strconv"
  "strings"
  "net/http"
  "io/ioutil"
  "encoding/json"
)

func DaemonManager() {
  for {
    say.Info("Manager Daemon: TicTac")
    go CheckRepos()
    go CheckTags()
    go CheckManifests()
    go CheckParents()
    time.Sleep(5 * 60 * time.Second)
  }
}

func IsSliceDifferent(a []string, b []string) (bool) {
  al := len(a)
  bl := len(b)
  if a == nil && b == nil {
    say.Info("Slices are equally nill. Same.")
    return false
  }
  if a == nil || b == nil {
    say.Info("One of the slices is empty. Different.")
    return true
  }
  if al != bl {
    say.Info("Length of slices are different. Different.")
    return true
  }
  numofequal := 0
  for _, bel := range b {
    for _, ael := range a {
      if bel == ael{
        numofequal++
        break
      }
    }
  }
  if len(a) == numofequal {
    say.Info("Length of slices are same with number of equal elements. Same.")
    return false
  } else {
    say.Info("Length of slices are differ with number of equal elements. Different.")
    return true
  }
}

func CheckRepos(){
  say.Info("CheckRepos Daemon: started work")
  repos := db.GetRepos()
  for _, e := range repos {
    pretty := db.GetRepoPretty(e)
    Req := "https://" + pretty["repouser"] +
      ":" + pretty["repopass"] + "@" + pretty["repohost"] + "/v2/_catalog?n=&last="
    if body, ok := MakeQueryToRepo(Req); ok {
      dbcatalog := db.GetCatalog(e)
      arrint := body.(map[string]interface{})["repositories"].([]interface{})
      arrstr := make([]string, len(arrint))
      for i, _ := range arrint {
        arrstr[i] = arrint[i].(string)
      }
      if IsSliceDifferent(dbcatalog, arrstr) {
        db.AddCatalog(e, arrstr)
      }
    } else {
      say.Error("CheckRepos Daemon: cannot recieve response from registry, stopping work")
    }
  }
  say.Info("CheckRepos Daemon: finished work")
}

func CheckTags(){
  say.Info("CheckTags Daemon: started work")
  repos := db.GetRepos()
  for _, er := range repos {
    pretty := db.GetRepoPretty(er)
    catalog := db.GetCatalog(er)
    reponame := "https://" + pretty["repouser"] + ":" + pretty["repopass"] + "@" + pretty["repohost"]
    for _, en := range catalog {
      Reqt := reponame + "/v2/" + en + "/tags/list"
      if body, ok := MakeQueryToRepo(Reqt); ok {
        dbtags := db.GetTags(er, en)
        arrint := body.(map[string]interface{})["tags"].([]interface{})
        arrstr := make([]string, len(arrint))
        for i, _ := range arrint {
          arrstr[i] = arrint[i].(string)
        }
        if IsSliceDifferent(dbtags, arrstr) {
          db.AddTags(er, en, arrstr)
        }
      } else {
        say.Error("CheckTags Daemon: cannot recieve response from registry, stopping work")
      }
    }
  }
  say.Info("CheckTags Daemon: finished work")
}

func CheckManifests(){
  say.Info("CheckManifests Daemon: started work")
  repos := db.GetRepos()
  for _, er := range repos {
    pretty := db.GetRepoPretty(er)
    catalog := db.GetCatalog(er)
    curlpath := "https://" + pretty["repouser"] + ":" + pretty["repopass"] + "@" + pretty["repohost"]
    for _, en := range catalog {
      dbtags := db.GetTags(er, en)
      for _, et := range dbtags {
        Reqt := curlpath + "/v2/" + en + "/manifests/" + et
        if body, ok := MakeQueryToRepo(Reqt); ok {
          client := &http.Client{}
          Reqtv2Digest, _ := http.NewRequest("GET", Reqt, nil)
          Reqtv2Digest.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
          if Respv2Digest, err := client.Do(Reqtv2Digest); err != nil {
            say.Error(err.Error())
            say.Error("CheckManifests Daemon: cannot recieve response from registry, stopping work")
          } else {
            defer Respv2Digest.Body.Close()
            dbdigest := db.GetTagDigest(er, en, et)
            curldigest := Respv2Digest.Header.Get("Docker-Content-Digest")
            if (dbdigest != curldigest){
              var ch interface{}
              totalsize := 0
              fsshaarr := body.(map[string]interface{})["fsLayers"].([]interface{})
              historyarr := body.(map[string]interface{})["history"].([]interface{})
              db.DeleteTagSubBucket(er, en, et, "history")
              for i, _ := range fsshaarr {
                fssha := fsshaarr[i].(map[string]interface{})["blobSum"].(string)
                fssize := GetfsLayerSize(curlpath + "/v2/" + en + "/blobs/" + fssha)
                history := historyarr[i].(map[string]interface{})["v1Compatibility"].(string)
                historynew := history
                if fsshanum, err := strconv.Atoi(fssize); err != nil {
                  say.Error(err.Error())
                } else {
                  if last := len(historynew) - 1; last >= 0 {
                      historynew = historynew[:last]
                  }
                  historynew = historynew + ",\"blobSum\":\"" + fssha + "\", \"blobSize\":\"" + fromByteToHuman(fsshanum) + "\"}"
                  totalsize += fsshanum
                }
                if err := json.Unmarshal([]byte(history), &ch); err != nil {
                  say.Error(err.Error())
                } else {
                  created := ch.(map[string]interface{})["created"].(string)
                  created = created[0:10] + " " + created[11:len(created)-11]
                  db.PutSimplePairToBucket([]string{ er, "catalog", en, et, "history" }, created, historynew)
                }
              }
              sizedt := time.Now().Local().Format("2006-01-02 15:04:05")
              db.PutSimplePairToBucket([]string{ er, "catalog", en, et, "_totalsizehuman" }, sizedt, fromByteToHuman(totalsize))
              db.PutSimplePairToBucket([]string{ er, "catalog", en, et, "_totalsizebytes" }, sizedt, strconv.Itoa(totalsize))
              db.PutTagDigest(er, en, et, curldigest)
            } else {
              say.Info("CheckManifests Daemon: digests are the same, shouldnot update anything, stopping work")
            }
          }
        } else {
          say.Error("CheckManifests Daemon: cannot recieve response from registry, stopping work")
        }
      }
    }
  }
  say.Info("CheckManifests Daemon: finished work")
}

func CheckParents(){
  repos := db.GetSimplePairsFromBucket([]string{})
  for key, value := range repos {
    if value == "" {
      names := db.GetSimplePairsFromBucket([]string{key, "catalog"})
      for keyn, valuen := range names {
        if valuen == "" {
          tags := db.GetSimplePairsFromBucket([]string{key, "catalog", keyn})
          for keyt, valuet := range tags {
            if (valuet == "") && (keyt[0:1] != "_"){
              history := db.GetSimplePairsFromBucket([]string{key, "catalog", keyn, keyt, "history"})
              histarr := []string{}
              var tmpstr string
              cmd := db.GetSimplePairsFromBucket([]string{key, "_names", keyn + ":" + keyt})
              for _, valh := range history {
                var ch interface{}
                if err := json.Unmarshal([]byte(valh), &ch); err != nil {
                  say.Error(err.Error())
                } else {
                  tmpstr = ""
                  for valji, valj := range ch.(map[string]interface{})["container_config"].(map[string]interface{})["Cmd"].([]interface{}) {
                    if strings.Contains(valj.(string), " CMD ") ||
                       strings.Contains(valj.(string), " WORKDIR ") ||
                       strings.Contains(valj.(string), " ENTRYPOINT ") ||
                       strings.Contains(valj.(string), " VOLUME ") ||
                       strings.Contains(valj.(string), " EXPOSE "){
                       tmpstr = ""
                       break
                    } else {
                      tmpstr += valj.(string)
                      if (valji < len(ch.(map[string]interface{})["container_config"].(map[string]interface{})["Cmd"].([]interface{}))-1) {
                        tmpstr += " "
                      }
                    }
                  }
                  if tmpstr != "" {
                    histarr = append(histarr, tmpstr)
                  }
                }
              }
              var cmdslice []string
              cmdneedaddition := true
              for _, valcmd := range cmd {
                if err := json.Unmarshal([]byte(valcmd), &cmdslice); err != nil {
                  say.Error(err.Error())
                } else {
                  if ! IsSliceDifferent(histarr, cmdslice) {
                    cmdneedaddition = false
                    break
                  }
                }
              }
              if cmdneedaddition {
                sizedt := time.Now().Local().Format("2006-01-02 15:04:05")
                fullcmd, _ := json.Marshal(histarr)
                db.PutSimplePairToBucket([]string{ key, "_names", keyn + ":" + keyt }, sizedt, string(fullcmd))
              }
              say.Info("Finding parent for [ " + keyn + ":" + keyt +  " ]")
              if pn, pt, pok := FindParent(histarr, key, keyn, keyt); pok {
                db.PutSimplePairToBucket([]string{ key, "catalog", keyn, keyt, "_parent" }, "name", pn)
                db.PutSimplePairToBucket([]string{ key, "catalog", keyn, keyt, "_parent" }, "tag",  pt)
              } else {
                db.PutSimplePairToBucket([]string{ key, "catalog", keyn, keyt, "_parent" }, "name", "")
                db.PutSimplePairToBucket([]string{ key, "catalog", keyn, keyt, "_parent" }, "tag",  "")
              }
            }
          }
        }
      }
    }
  }
}

func GetfsLayerSize(link string ) (size string){
  if Resp, err := http.Head(link); err != nil {
    say.Error(err.Error())
    say.Error("CheckManifests Daemon: GetfsLayerSize cannot recieve response from registry, stopping work")
  } else {
    defer Resp.Body.Close()
    if _, err := ioutil.ReadAll(Resp.Body); err != nil {
      say.Error(err.Error())
    } else {
      size = Resp.Header.Get("Content-Length")
      return
    }
  }
  return ""
}

func fromByteToHuman(bytes int) (human string){
  human = strconv.Itoa(bytes) + " B"
  if bytes > 1024 {
    bytes = bytes / 1024
    human = strconv.Itoa(bytes) + " KB"
  }
  if bytes > 1024 {
    bytes = bytes / 1024
    human = strconv.Itoa(bytes) + " MB"
  }
  if bytes > 1024 {
    bytes = bytes / 1024
    human = strconv.Itoa(bytes) + " GB"
  }
  return
}

func DeleteTagFromRepo(repo string, name string, tag string) (ok bool){
  ok = false
  pretty := db.GetRepoPretty(repo)
  curlpath := "https://" + pretty["repouser"] + ":" + pretty["repopass"] + "@" + pretty["repohost"]
  ReqtStr := curlpath + "/v2/" + name + "/manifests/" + db.GetValueFromBucket([]string{repo, "catalog", name, tag}, "digest")
  client := &http.Client{}
  Reqt, _ := http.NewRequest("DELETE", ReqtStr, nil)
  Reqt.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
  if Resp, err := client.Do(Reqt); err != nil {
    say.Error(err.Error())
    say.Error("Delete From Repository: cannot recieve response from registry, stopping work")
    return
  } else {
    defer Resp.Body.Close()
    if Resp.StatusCode == 202 {
      ok = true
    } else {
      say.Error(ReqtStr)
      say.Error(Resp.Status)
    }
  }
  return
}

func MakeQueryToRepo(query string) (body interface{}, ok bool){
  ok = false
  if response, err := http.Get(query); err != nil {
    say.Error(err.Error())
    return
  } else {
    defer response.Body.Close()
    if bodytmp, err := ioutil.ReadAll(response.Body); err != nil {
      say.Error(err.Error())
      return
    } else {
      var c interface{}
      if err := json.Unmarshal(bodytmp, &c); err != nil {
        say.Error(err.Error())
        return
      } else {
        if c.(map[string]interface{})["errors"] != nil {
          say.Error(query)
          say.Error(c.(map[string]interface{})["errors"].([]interface{})[0].(map[string]interface{})["message"].(string))
          return
        } else {
          body = c
          ok = true
        }
      }
    }
  }
  return
}

func FindParent(childcmd []string, repo string, namei string, tagi string) (name string, tag string, ok bool){
  say.Info("Searching for parent of [ " + namei + ":" + tagi + " ]")
  ok = true
  names := db.GetSimplePairsFromBucket([]string{repo, "_names"})
  maxname := ""
  maxlayers := 0
  for kn, _ := range names {
    if strings.Split(kn, ":")[0] != namei {
      cmd := db.GetSimplePairsFromBucket([]string{repo, "_names", kn})
      for _, vc := range cmd {
        var parentcmd interface{}
        if err := json.Unmarshal([]byte(vc), &parentcmd); err == nil {
          includecount := 0
          for _, childraw := range childcmd {
            cmdinparent := false
            for _, parentraw := range parentcmd.([]interface{}) {
              if parentraw == childraw {
                cmdinparent = true
                break
              }
            }
            if cmdinparent {
              includecount++
            }
          }
          if includecount == len(parentcmd.([]interface{})) {
            if len(parentcmd.([]interface{})) < len(childcmd) {
              if maxlayers < len(parentcmd.([]interface{})) {
                maxlayers = len(parentcmd.([]interface{}))
                maxname = kn
              }
            }
          }
        } else {
          say.Error(err.Error())
          ok = false
          return
        }
      }
    }
  }
  if maxlayers == 0 {
    ok = false
    say.Info("Parent not found")
  } else {
    say.Info("Parent is [ "+ maxname +" ]")
    s := strings.Split(maxname, ":")
    name = s[0]
    tag = s[1]
  }
  return
}
