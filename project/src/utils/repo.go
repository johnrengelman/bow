package utils

import(
  "db"
  "say"
  "strconv"
  "net/http"
  "io/ioutil"
  "encoding/json"
)

func DeleteTagFromRepo(repo string, name string, tag string) (ok bool){
  ok = false
  pretty := db.GetRepoPretty(repo)
  curlpath := pretty["reposcheme"] + "://" + pretty["repouser"] + ":" + pretty["repopass"] + "@" + pretty["repohost"]
  ReqtStr := curlpath + "/v2/" + name + "/manifests/" + db.GetValueFromBucket([]string{repo, "catalog", name, tag}, "digest")
  client := &http.Client{}
  Reqt, _ := http.NewRequest("DELETE", ReqtStr, nil)
  Reqt.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
  if Resp, err := client.Do(Reqt); err != nil {
    say.L3(err.Error())
    say.L3("Delete From Repository: cannot recieve response from registry, stopping work")
    return
  } else {
    defer Resp.Body.Close()
    if Resp.StatusCode == 202 {
      ok = true
    } else {
      if Resp.StatusCode == 405 {
        say.L3(Resp.Status)
        say.L3("You need to add '-e REGISTRY_STORAGE_DELETE_ENABLED=true'")
        say.L3("Follow instructions here: https://github.com/Evedel/bow#image-deletion")
      } else {
        say.L3("Delete manifest: " + Resp.Status)
      }
      say.L3(ReqtStr)
    }
  }
  return
}

func MakeQueryToRepo(query string) (body interface{}, ok bool){
  ok = false
  if response, err := http.Get(query); err != nil {
    body = -1
    say.L3(err.Error())
    say.L3("Probably something wrong with network configuration or registry state")
  } else {
    defer response.Body.Close()
    if response.StatusCode == 200 {
      if bodytmp, err := ioutil.ReadAll(response.Body); err != nil {
        say.L3(err.Error())
      } else {
        var c interface{}
        if err := json.Unmarshal(bodytmp, &c); err != nil {
          say.L3(err.Error())
        } else {
          if c.(map[string]interface{})["errors"] != nil {
            say.L3("Query :" + query)
            say.L3(c.(map[string]interface{})["errors"].([]interface{})[0].(map[string]interface{})["message"].(string))
          } else {
            body = c
            ok = true
          }
        }
      }
    } else {
      body = response.StatusCode
      switch response.StatusCode {
      case 401: say.L3("[401] : Unauthorized response is returned (credentials problem)")
      default:  say.L3("Cannot diagnose error: \n[ " + strconv.Itoa(response.StatusCode) + " ] " + response.Status)
      }
    }
  }
  return
}

func GetfsLayerSize(link string ) (size string){
  if Resp, err := http.Head(link); err != nil {
    say.L3(err.Error())
    say.L3("GetfsLayerSize: Cannot recieve response from registry, stopping work")
  } else {
    defer Resp.Body.Close()
    if _, err := ioutil.ReadAll(Resp.Body); err != nil {
      say.L3(err.Error())
    } else {
      size = Resp.Header.Get("Content-Length")
      return
    }
  }
  return ""
}
