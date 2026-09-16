library(dplyr)
library(optparse)
library(readr)
library(blastula)
option_list = list(make_option(c("--jobid"), type = "character", default = NULL, help = "file, provided by default."),
                   make_option(c("--step"), type = "character", default = NULL, help = "_human.txt, provided by default."),
                   make_option(c("--send_to"), type = "character", default = NULL, help = "_human.txt, provided by default."))
args = commandArgs(trailingOnly=F)
args <- parse_args(OptionParser(option_list = option_list))
# 获取命令行参数
jobid <- args$jobid
step <- args$step
user_email <- args$send_to

beavermail <- function(jobid,step,send_from = "BeaversBot@outlook.com",send_to,creds_files){
  library(blastula)
  date_time <- blastula::add_readable_time()
  email <- blastula::compose_email(
    body = blastula::md(glue::glue(
      "Hello,Dear USERs!
your Job {jobid} is running in the BeaverBoard, and step {step} is done!

Best wishes,

BeaverBoard Team.
")),
    footer = blastula::md(glue::glue("Email sent on {date_time}."))
  )
  email %>%
    blastula::smtp_send(
      to = send_to,
      from = send_from,
      subject = glue::glue("Process Report : step {step} is done."),
      credentials = creds_file(creds_files)
    )
}

message(">> Warning that email function is depressed when the cred file is not presented.")
if(file.exists("zyh163_creds2")){
  message("creds file presented")
}
try(beavermail(jobid = jobid,
           step = step,
           send_from = "BeaversBot@outlook.com",
           send_to = user_email,
           creds_files = "zyh163_creds2"))
